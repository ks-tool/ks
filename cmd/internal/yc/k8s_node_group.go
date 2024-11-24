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
	"strconv"

	"github.com/ks-tool/ks/apis/scheme"
	ycv1alpha1 "github.com/ks-tool/ks/apis/yc/v1alpha1"
	"github.com/ks-tool/ks/pkg/common"
	"github.com/ks-tool/ks/pkg/utils"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1/instancegroup"
	"github.com/yandex-cloud/go-sdk/operation"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	k8sNodeGroup = &cobra.Command{
		Aliases: []string{"ng"},
		Use:     "node-group",
		Short:   "Manage Kubernetes node groups",
	}

	k8sNodeGroupAdd = &cobra.Command{
		Use:   "add <cluster-name>",
		Short: "Add a new node group to an existing Kubernetes cluster",
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
			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			cluster, err := getCluster(cmd.Context(), client, args[0])
			if err != nil {
				log.Fatal(err)
			}

			var manifest *ycv1alpha1.ComputeInstance
			manifestPath := viper.GetString("manifest")
			if len(manifestPath) > 0 {
				obj, err := scheme.FromFileWithDefaults(manifestPath)
				if err != nil {
					log.Fatal(err)
				}

				var ok bool
				manifest, ok = obj.(*ycv1alpha1.ComputeInstance)
				if !ok {
					log.Fatal("expected ComputeInstance kind of manifest, got " +
						obj.GetObjectKind().GroupVersionKind().Kind)
				}
			} else {
				manifest, err = computeInstanceFromFlags(client)
				if err != nil {
					log.Fatal(err)
				}
			}

			if len(manifest.Spec.UserData) == 0 {
				ud, err := renderUserDataTemplate(common.UserDataVMTemplate)
				if err != nil {
					log.Fatal(err)
				}
				manifest.Spec.UserData = ud
			}

			req := new(instancegroup.CreateInstanceGroupRequest)
			if err := convert(manifest, req, client); err != nil {
				log.Fatal(err)
			}

			req.Labels[common.LabelClusterKey] = cluster.master.Id
			req.InstanceTemplate.Labels[common.LabelClusterKey] = cluster.master.Id
			replicas := viper.GetUint("replicas")
			if replicas > 0 {
				req.ScalePolicy.ScaleType.(*instancegroup.ScalePolicy_FixedScale_).FixedScale.Size = int64(replicas)
			}

			op, err := client.InstanceGroupCreate(cmd.Context(), req)
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}

	k8sNodeGroupDelete = &cobra.Command{
		Aliases: []string{"rm", "del"},
		Use:     "delete <node-group> [node-group ...]",
		Short:   "Delete the node group",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				return err
			}

			return viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			var op *operation.Operation
			for _, id := range args {
				if op, err = client.InstanceGroupDelete(cmd.Context(), id); err != nil {
					log.Fatal(err)
				}
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}

	k8sNodeGroupScale = &cobra.Command{
		Use:   "scale <node-group>",
		Short: "Scale node group",
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

			groupId := args[0]
			lst, err := client.InstanceGroupInstancesList(cmd.Context(), groupId)
			if err != nil {
				log.Fatal(err)
			}
			curSize := len(lst)

			delNodes := viper.GetStringSlice("delete-node")
			size := len(delNodes)

			replicas := viper.GetString("replicas")
			if len(replicas) > 0 {
				var op string
				if replicas[0] == '+' || replicas[0] == '-' {
					op = string(replicas[0])
					replicas = replicas[1:]
				}

				size, err = strconv.Atoi(replicas)
				if err != nil {
					log.Fatal(err)
				}

				if op == "+" {
					size = curSize + size
				} else if op == "-" {
					size = curSize - size
					if size < 0 {
						size = 0
					}
				}
			}

			var managedInstanceIds []string
			if size < curSize {
				n := curSize - size
				for _, item := range lst {
					if len(delNodes) == 0 && len(managedInstanceIds) == n {
						break
					}
					if len(delNodes) > 0 && !utils.In(delNodes, item.InstanceId) && !utils.In(delNodes, item.Name) {
						continue
					}
					managedInstanceIds = append(managedInstanceIds, item.Id)
				}

				if _, err = client.InstanceGroupInstancesDelete(cmd.Context(), groupId, managedInstanceIds...); err != nil {
					log.Fatal(err)
				}

			} else if size == curSize {
				return
			}

			op, err := client.InstanceGroupScale(cmd.Context(), groupId, size)
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	k8sCmd.AddCommand(k8sNodeGroup)

	k8sNodeGroup.AddCommand(
		k8sNodeGroupAdd,
		k8sNodeGroupDelete,
		k8sNodeGroupScale,
	)

	computeCreateFlags(k8sNodeGroupAdd)
	k8sNodeGroupNew(k8sNodeGroupAdd)
	noWait(k8sNodeGroupAdd)
	noWait(k8sNodeGroupDelete)
	k8sNodeGroupScaleFlags(k8sNodeGroupScale)
	noWait(k8sNodeGroupScale)
}

func k8sNodeGroupScaleFlags(cmd *cobra.Command) {
	cmd.Flags().String("replicas", "", "number of worker node replicas")
	cmd.Flags().StringSlice("delete-node", nil, "scale down workers and delete specific node")
	cmd.MarkFlagsMutuallyExclusive("delete-node", "replicas")
	cmd.MarkFlagsOneRequired("delete-node", "replicas")
}

func k8sNodeGroupNew(cmd *cobra.Command) {
	cmd.Flags().Uint("replicas", 0, "number of worker node replicas")
	_ = cmd.MarkFlagRequired("sa")
}
