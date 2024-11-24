/*
 Copyright (c) 2024 Alexey Shulutkov <github@shulutkov.ru>

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package yc

import (
	"context"
	"errors"
	"fmt"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1/instancegroup"
	"sync"

	"github.com/ks-tool/ks/apis/scheme"
	ycv1alpha1 "github.com/ks-tool/ks/apis/yc/v1alpha1"
	"github.com/ks-tool/ks/pkg/common"
	"github.com/ks-tool/ks/pkg/yc"

	computev1 "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-sdk/operation"

	"github.com/ks-tool/ks/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	k8sCmd = &cobra.Command{
		Use:   "k8s",
		Short: "Manage scratch Kubernetes clusters",
	}

	k8sClusterCreate = &cobra.Command{
		Use:   "create <name>",
		Short: "Create a Kubernetes cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			_ = viper.BindPFlags(cmd.Flags())
			bind(cmd, "cpu", "resources.cpu")
			bind(cmd, "core-fraction", "resources.core-fraction")
			bind(cmd, "memory", "resources.memory")
			bind(cmd, "disk-type", "disk.type")
			bind(cmd, "disk-image", "disk.image")
			bind(cmd, "disk-size", "disk.size")

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			var manifest *ycv1alpha1.Kubernetes
			manifestPath := viper.GetString("manifest")
			if len(manifestPath) > 0 {
				obj, err := scheme.FromFileWithDefaults(manifestPath)
				if err != nil {
					log.Fatal(err)
				}

				var ok bool
				manifest, ok = obj.(*ycv1alpha1.Kubernetes)
				if !ok {
					log.Fatal("expected Kubernetes kind of manifest, got " +
						obj.GetObjectKind().GroupVersionKind().Kind)
				}
			} else {
				manifest = new(ycv1alpha1.Kubernetes)
				obj, err := computeInstanceFromFlags(client)
				if err != nil {
					log.Fatal(err)
				}

				manifest.ObjectMeta = obj.ObjectMeta
				manifest.Spec.ControlPlain = obj.Spec
			}

			if len(manifest.Spec.ControlPlain.UserData) == 0 {
				ud, err := renderUserDataTemplate(common.UserDataK8sControlPlainTemplate)
				if err != nil {
					log.Fatal(err)
				}
				manifest.Spec.ControlPlain.UserData = ud
			}

			req := new(ycv1alpha1.KubernetesRequest)
			if err = convert(manifest, req, client); err != nil {
				log.Fatal(err)
			}

			req.ControlPlain.Labels[common.LabelNodeRoleControlPlane] = ""
			req.ControlPlain.Name = name

			op, err := client.ComputeInstanceCreate(cmd.Context(), req.ControlPlain)
			if err != nil {
				log.Fatal(err)
			}

			meta, err := op.Metadata()
			if err != nil {
				log.Fatal(err)
			}

			newComputeInstance := meta.(*computev1.CreateInstanceMetadata)
			log.Infof("Creating Kubernetes cluster %s (%s) ...", name, newComputeInstance.InstanceId)

			for _, ng := range req.NodeGroups {
				if _, err = client.InstanceGroupCreate(cmd.Context(), ng); err != nil {
					log.Fatal(err)
				}
			}

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}

			resp, err := op.Response()
			if err != nil {
				log.Fatal(err)
			}

			instance := resp.(*computev1.Instance)

			ip := yc.GetIPv4(instance).PublicOrPrivate(true)
			log.Infof("The Kubernetes cluster %q (%s) created", name, ip)
		},
	}

	k8sClusterGet = &cobra.Command{
		Use:   "get [name]",
		Short: "Display one or many Kubernetes clusters",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			var clusters []cluster
			var err error
			if len(args) == 0 {
				clusters, err = listClusters(cmd.Context())
			} else {
				clusters, err = getClusters(cmd.Context(), args...)
			}
			if err != nil {
				log.Fatal(err)
			}

			tbl := utils.NewTable()
			if !viper.GetBool("no-header") {
				tbl.SetHeader("ClusterID", "Name", "IP", "Workers", "Status", "ZoneID", "SubnetID", "Platform")
			}

			for _, c := range clusters {
				tbl.AddRow(
					c.master.Id,
					c.master.Name,
					yc.GetIPv4(c.master).PublicOrPrivate(false),
					len(c.workers),
					c.master.Status.String(),
					c.master.ZoneId,
					c.master.NetworkInterfaces[0].SubnetId,
					c.master.PlatformId,
				)
			}

			tbl.Render(cmd.OutOrStdout())
		},
	}

	k8sClusterDelete = &cobra.Command{
		Aliases: []string{"rm", "del"},
		Use:     "delete <cluster-name>",
		Short:   "Delete a Kubernetes cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			return viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			c, err := getCluster(cmd.Context(), client, args[0])
			if err != nil {
				log.Fatal(err)
			}

			log.Infof("Delete Kubernetes cluster %s (%s) ...", c.master.Name, c.master.Id)
			for _, w := range c.workers {
				log.Infof("Deleting worker %q", w.Id)
				if _, err := client.ComputeInstanceDelete(cmd.Context(), w.Id); err != nil {
					log.Fatal(err)
				}
			}

			log.Infof("Deleting master %q", c.master.Name)
			op, err := client.ComputeInstanceDelete(cmd.Context(), c.master.Id)
			if err != nil {
				log.Fatal(err)
			}
			if err := op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}

	k8sClusterStart = &cobra.Command{
		Use:   "start <cluster-name>",
		Short: "Start a Kubernetes cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			return viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			c, err := getCluster(cmd.Context(), client, args[0])
			if err != nil {
				log.Fatal(err)
			}

			var op *operation.Operation
			for _, w := range c.workers {
				log.Infof("Stopping worker %q", w.Id)
				op, err = client.ComputeInstanceStart(cmd.Context(), w.Id)
				if err != nil {
					log.Fatal(err)
				}
			}

			op, err = client.ComputeInstanceStart(cmd.Context(), c.master.Id)
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err := op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}

	k8sClusterStop = &cobra.Command{
		Use:   "stop <cluster-name>",
		Short: "Stop a Kubernetes cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			return viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			c, err := getCluster(cmd.Context(), client, args[0])
			if err != nil {
				log.Fatal(err)
			}

			var op *operation.Operation
			for _, w := range c.workers {
				log.Infof("Stopping worker %q", w.Id)
				op, err = client.ComputeInstanceStop(cmd.Context(), w.Id)
				if err != nil {
					log.Fatal(err)
				}
			}

			op, err = client.ComputeInstanceStop(cmd.Context(), c.master.Id)
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err := op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}

	k8sClusterScale = &cobra.Command{
		Use:   "scale <cluster> <node-group>",
		Short: "Scale Kubernetes workers",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(2)(cmd, args); err != nil {
				return err
			}

			return viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			c, err := getCluster(cmd.Context(), client, args[0])
			if err != nil {
				log.Fatal(err)
			}

			var op *operation.Operation
			replicas := viper.GetUint("replicas")
			if cmd.Flags().Changed("replicas") && replicas == 0 {
				return
			}
			if replicas > 0 {
				if len(c.workers) < int(replicas) {
					//up

				} else if len(c.workers) > int(replicas) {
					// down
					if len(c.workers) == 0 {
						return
					}

					n := len(c.workers) - int(replicas)
					for _, w := range c.workers[:n-1] {
						log.Infof("Deleting worker %q", w.Id)
						if op, err = client.ComputeInstanceDelete(cmd.Context(), w.Id); err != nil {
							log.Fatal(err)
						}
					}
				}
			}

			for _, node := range viper.GetStringSlice("delete-node") {
				log.Infof("Deleting worker %q", node)
				if op, err = client.ComputeInstanceDelete(cmd.Context(), node); err != nil {
					log.Fatal(err)
				}
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err := op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}
)

// K8s represents the k8s command
func init() {
	YandexCloudCmd.AddCommand(k8sCmd)

	k8sCmd.AddCommand(
		k8sClusterCreate,
		k8sClusterDelete,
		k8sClusterGet,
		k8sClusterStart,
		k8sClusterStop,
		k8sClusterScale,
	)

	computeCreateFlags(k8sClusterCreate)
	computeNoPublicIp(k8sClusterCreate)
	k8sGetFlags(k8sClusterGet)
	noWait(k8sClusterCreate)
	noWait(k8sClusterDelete)
	noWait(k8sClusterStart)
	noWait(k8sClusterStop)
	computeCreateFlags(k8sClusterScale)
	noWait(k8sClusterScale)
}

func k8sGetFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("no-header", false, "don't print headers")
}

func noWait(cmd *cobra.Command) {
	cmd.Flags().Bool("no-wait", false, "don't wait for completion")
}

type cluster struct {
	master  *computev1.Instance
	workers []*instancegroup.InstanceGroup
}

func listClusters(ctx context.Context) ([]cluster, error) {
	client, err := newClient()
	if err != nil {
		return nil, err
	}

	clusters, err := getMaster(ctx, client)
	if err != nil {
		return nil, err
	}

	lbl := map[string]string{common.ManagedKey: common.KsToolKey}
	nodeGroups, err := client.InstanceGroupList(ctx, "", lbl)
	if err != nil {
		return nil, err
	}

	out := make([]cluster, len(clusters))
	for n, c := range clusters {
		cl := cluster{
			master:  c,
			workers: make([]*instancegroup.InstanceGroup, 0),
		}
		for _, ng := range nodeGroups {
			if ng.Labels[common.LabelClusterKey] == c.Id {
				cl.workers = append(cl.workers, ng)
			}
		}
		out[n] = cl
	}

	return out, nil
}

func getMaster(ctx context.Context, client *yc.Client, name ...string) ([]*computev1.Instance, error) {
	lbl := map[string]string{common.ManagedKey: common.KsToolKey, common.LabelNodeRoleControlPlane: ""}
	var filter []yc.Filter
	if len(name) == 1 {
		filter = append(filter, yc.Filter{
			Field:    "name",
			Operator: yc.OperatorEq,
			Value:    name[0],
		})
	}

	lst, err := client.ComputeInstanceList(ctx, "", lbl, filter...)
	if err != nil {
		return nil, err
	}
	if len(lst) == 0 {
		return nil, fmt.Errorf("cluster %q not found", name)
	}

	if len(name) > 1 {
		var out []*computev1.Instance
		for _, item := range lst {
			if item.Name == name[0] {
				out = append(out, item)
			}
		}
		return out, nil
	}

	return lst, nil
}

func getCluster(ctx context.Context, client *yc.Client, name string) (cluster, error) {
	m, err := getMaster(ctx, client, name)
	if err != nil {
		return cluster{}, err
	}
	if len(m) > 1 {
		return cluster{}, fmt.Errorf("find more than one cluster with name %q", name)
	}

	lbl := map[string]string{common.LabelClusterKey: m[0].Id}
	w, err := client.InstanceGroupList(
		ctx, "",
		lbl,
	)
	if err != nil {
		return cluster{}, err
	}

	return cluster{master: m[0], workers: w}, nil
}

func getClusters(ctx context.Context, names ...string) ([]cluster, error) {
	wg := &sync.WaitGroup{}
	clusters := make([]cluster, len(names))
	client, err := newClient()
	if err != nil {
		return nil, err
	}

	var errs []error
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()

			c, e := getCluster(ctx, client, name)
			if e != nil {
				errs = append(errs, e)
				return
			}

			clusters[i] = c
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	if len(clusters) == 0 {
		return nil, errors.New("no clusters found")
	}

	return clusters, nil
}

func createNComputeInstance(ctx context.Context, n uint, client *yc.Client, req *computev1.CreateInstanceRequest) (*operation.Operation, error) {
	var err error
	var op *operation.Operation
	var i uint
	for i = 0; i < n; i++ {
		op, err = client.ComputeInstanceCreate(ctx, req)
		if err != nil {
			return nil, err
		}

		meta, err := op.Metadata()
		if err != nil {
			return nil, err
		}
		log.Infof("Creating new worker %q ...", meta.(*computev1.CreateInstanceMetadata).InstanceId)
	}

	return op, nil
}
