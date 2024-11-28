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
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/ks-tool/ks/apis/scheme"
	"github.com/ks-tool/ks/apis/scopes"
	ycv1alpha1 "github.com/ks-tool/ks/apis/yc/v1alpha1"
	"github.com/ks-tool/ks/pkg/common"
	"github.com/ks-tool/ks/pkg/utils"
	"github.com/ks-tool/ks/pkg/yc"

	computev1 "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-sdk/operation"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	computeCmd = &cobra.Command{
		Aliases: []string{"vm"},
		Use:     "compute",
		Short:   "Manage Compute Cloud",
	}

	vmCreate = &cobra.Command{
		Use:   "create [name]",
		Short: "Create a compute instance",
		PreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlags(cmd.Flags())
			bind(cmd, "cpu", "resources.cpu")
			bind(cmd, "core-fraction", "resources.core-fraction")
			bind(cmd, "memory", "resources.memory")
			bind(cmd, "disk-type", "disk.type")
			bind(cmd, "disk-image", "disk.image")
			bind(cmd, "disk-size", "disk.size")
		},
		Run: func(cmd *cobra.Command, args []string) {
			var name string
			if len(args) > 0 {
				name = args[0]
			}

			client, err := newClient()
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

			req := new(computev1.CreateInstanceRequest)
			if err := convert(manifest, req, client); err != nil {
				log.Fatal(err)
			}
			req.Name = name

			op, err := client.ComputeInstanceCreate(cmd.Context(), req)
			if err != nil {
				log.Fatal(err)
			}

			meta, err := op.Metadata()
			if err != nil {
				log.Fatal(err)
			}

			newComputeInstance := meta.(*computev1.CreateInstanceMetadata)
			log.Infof("Creating compute instance %s ...", newComputeInstance.InstanceId)

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}

			resp, err := op.Response()
			if err != nil {
				log.Fatal(err)
			}

			instance := resp.(*computev1.Instance)
			if len(name) == 0 {
				name = instance.Id
			}

			ip := yc.GetIPv4(instance).PublicOrPrivate(true)
			log.Infof("The compute instance %s (%s) created", name, ip)
		},
	}

	vmGet = &cobra.Command{
		Use:   "get [name ...]",
		Short: "Display one or many compute instances",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newClient()
			if err != nil {
				log.Fatal(err)
			}

			var filters []yc.Filter
			if viper.IsSet("status") {
				status := viper.GetString("status")
				if !yc.AllowStatus(status) {
					log.Fatalf("invalid status %q", status)
				}
				filters = append(filters, yc.Filter{
					Field:    "status",
					Operator: yc.OperatorEq,
					Value:    strings.ToUpper(status),
				})
			}

			if cmd.Flags().Changed("zone") {
				filters = append(filters, yc.Filter{
					Field:    "zoneId",
					Operator: yc.OperatorEq,
					Value:    viper.GetString("zone"),
				})
			}

			var lbl map[string]string
			if !viper.GetBool("all") {
				lbl = map[string]string{common.ManagedKey: common.KsToolKey}
			}
			lst, err := client.ComputeInstanceList(cmd.Context(), "", lbl, filters...)
			if err != nil {
				log.Fatal(err)
			}

			tbl := utils.NewTable()
			if !viper.GetBool("no-header") {
				tbl.SetHeader("ID", "Name", "IP", "Status", "ZoneID", "SubnetID", "Platform")
			}

			for _, item := range lst {
				if len(args) > 0 && !utils.In(args, item.Name) {
					continue
				}
				tbl.AddRow(
					item.Id,
					item.Name,
					yc.GetIPv4(item).PublicOrPrivate(false),
					item.Status.String(),
					item.ZoneId,
					item.NetworkInterfaces[0].SubnetId,
					item.PlatformId,
				)
			}

			tbl.Render(cmd.OutOrStdout())
		},
	}

	vmDelete = &cobra.Command{
		Aliases: []string{"rm", "del"},
		Use:     "delete",
		Short:   "Delete a compute instance",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		Run: computeInstanceAction("delete"),
	}

	vmStart = &cobra.Command{
		Use:   "start",
		Short: "Start a compute instance",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		Run: computeInstanceAction("start"),
	}

	vmStop = &cobra.Command{
		Use:   "stop",
		Short: "Stop a compute instance",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		Run: computeInstanceAction("stop"),
	}

	vmUserDataShow = &cobra.Command{
		Aliases: []string{"ud"},
		Use:     "user-data",
		Short:   "Show user-data for a compute instance",
		PreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlags(cmd.Flags())
			cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
				if _, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok {
					_ = cmd.InheritedFlags().SetAnnotation(f.Name, cobra.BashCompOneRequiredFlag, []string{"false"})
				}
			})
		},
		Run: func(cmd *cobra.Command, args []string) {
			tpl, err := getUserData(common.UserDataVMTemplate)
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("raw") {
				cmd.Print(tpl)
				return
			}

			vars, err := templateVariables()
			if err != nil {
				log.Fatal(err)
			}

			ud, err := utils.Template(tpl, vars)
			if err != nil {
				log.Fatal(err)
			}

			cmd.Print(ud)
		},
	}
)

func init() {
	YandexCloudCmd.AddCommand(computeCmd)

	computeCmd.AddCommand(
		vmCreate,
		vmDelete,
		vmGet,
		vmStart,
		vmStop,
		vmUserDataShow,
	)

	computeCreateFlags(vmCreate)
	computeNoPublicIp(vmCreate)
	userDataFlags(vmCreate)
	actionFlags(vmDelete)
	actionFlags(vmStart)
	actionFlags(vmStop)
	vmGetFlags(vmGet)
	vmUserDataShowFlags(vmUserDataShow)
	userDataFlags(vmUserDataShow)

	computeCmd.PersistentFlags().String("zone", ycv1alpha1.DefaultZone, "zone for creating resources")
}

func computeCreateFlags(cmd *cobra.Command) {
	cmd.Flags().String("platform", "", "specific platform for the instance")
	cmd.Flags().String("sa", "", "service account name")
	cmd.Flags().Bool("preemptible", true, "creates preemptible instance")
	cmd.Flags().String("address", "", "assigns the given internal address to the instance that is created")
	cmd.Flags().Int64("cpu", 0, "specific number of CPU cores for an instance")
	cmd.Flags().Int64("core-fraction", 0, "specific baseline performance for a core in percent")
	cmd.Flags().Int64("memory", 0, "specific how much memory (in GiB) instance should have")
	cmd.Flags().String("disk-type", "", "the type of the disk")
	cmd.Flags().String("disk-image", "", "the source image used to create the disk")
	cmd.Flags().Int64("disk-size", 0, "the size of the disk in GiB")
	cmd.Flags().String("subnet", "", "specific the name of the subnet")
	cmd.Flags().String("manifest", "", "path to manifest file")
	cmd.Flags().StringSlice("labels", nil, "additional labels to add to the instance (e.g. --labels key1=value1,key2=value2)")
}

func computeNoPublicIp(cmd *cobra.Command) {
	cmd.Flags().Bool("no-public-ip", false, "do not use public IP for instance")
}

func userDataFlags(cmd *cobra.Command) {
	cmd.Flags().String("user", "", "create user with specific name. Use current username if not set ")
	cmd.Flags().String("shell", "/bin/bash", "set login shell for user")
	cmd.Flags().String("user-data-file", "", "custom user-data file")
	cmd.Flags().StringSlice("ssh-key", nil, "add public SSH key from specified file to authorized_keys")
}

func actionFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("no-wait", false, "don't wait for completion")
	cmd.Flags().String("id", "", "id of compute instance")
	cmd.Flags().String("name", "", "name of compute instance")
	cmd.MarkFlagsMutuallyExclusive("id", "name")
	cmd.MarkFlagsOneRequired("id", "name")
}

func vmGetFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "show all compute instances")
	cmd.Flags().Bool("no-header", false, "don't print headers")
	cmd.Flags().String("status", "", "show compute instances with specific status. Allow: "+yc.Statuses())
}

func vmUserDataShowFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("raw", false, "not render template")
}

func bind(cmd *cobra.Command, name string, key ...string) {
	viperKey := name
	if len(key) > 0 {
		viperKey = key[0]
	}
	_ = viper.BindPFlag(viperKey, cmd.Flags().Lookup(name))
}

func newClient() (*yc.Client, error) {
	var opts []yc.ClientConfig
	cloudName := viper.GetString("cloud")
	if viper.IsSet("cloud") {
		opts = append(opts, yc.CloudName(cloudName))
	}

	folderName := viper.GetString("folder")
	if viper.IsSet("folder") {
		opts = append(opts, yc.FolderName(folderName))
	}

	return yc.NewClient(viper.GetString("token"), opts...)
}

func getUserData(def string) (string, error) {
	filePath := viper.GetString("user-data-file")
	if len(filePath) > 0 {
		file, err := homedir.Expand(filePath)
		if err != nil {
			return "", err
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return def, nil
}

func templateVariables() (map[string]any, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	username := viper.GetString("user")
	if len(username) == 0 {
		username = usr.Username
	}

	keys := viper.GetStringSlice("ssh-key")
	if len(keys) == 0 {
		keys, err = utils.GetAllSshPublicKeys(usr.HomeDir)
	} else {
		for _, keyFile := range keys {
			keyFile, err = homedir.Expand(keyFile)
			if err != nil {
				return nil, err
			}
			b, err := os.ReadFile(keyFile)
			if err != nil {
				return nil, err
			}
			keys = append(keys, strings.TrimSpace(string(b)))
		}
	}

	out := map[string]any{
		"user":    username,
		"sshKeys": keys,
	}

	shell := viper.GetString("shell")
	if len(shell) > 0 {
		out["shell"] = shell
	}

	return out, nil
}

func renderUserDataTemplate(tpl string) (string, error) {
	vars, err := templateVariables()
	if err != nil {
		return "", err
	}

	tpl, err = getUserData(tpl)
	if err != nil {
		return "", err
	}

	return utils.Template(tpl, vars)
}

func computeInstanceFromFlags(client *yc.Client) (*ycv1alpha1.ComputeInstance, error) {
	obj := new(ycv1alpha1.ComputeInstance)
	if err := viper.Unmarshal(&obj.Spec); err != nil {
		return nil, err
	}

	if viper.IsSet("labels") {
		obj.Labels = make(map[string]string)
	}
	for _, l := range viper.GetStringSlice("labels") {
		lbl := strings.SplitN(l, "=", 2)
		if len(lbl) != 2 {
			return nil, fmt.Errorf("invalid label: %q", l)
		}
		obj.Labels[lbl[0]] = lbl[1]
	}

	subnet := viper.GetString("subnet")
	if len(subnet) == 0 {
		snet, err := client.VPCFirstSubnetInZone(context.Background(), "", viper.GetString("zone"))
		if err != nil {
			return nil, err
		}

		subnet = snet.Name
	}
	netif := ycv1alpha1.NetworkInterfaceSpec{
		Subnet:    subnet,
		PrivateIp: viper.GetString("address"),
	}
	if !viper.GetBool("no-public-ip") {
		pub := ""
		netif.PublicIp = &pub
	}

	obj.Spec.NetworkInterfaces = append(obj.Spec.NetworkInterfaces, netif)

	scheme.Defaults(obj)

	return obj, nil
}

func computeInstanceAction(action string) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		client, err := newClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := cmd.Context()
		instanceId := viper.GetString("id")
		instanceName := viper.GetString("name")
		if viper.IsSet("name") {
			lst, err := client.ComputeInstanceList(ctx, "", nil, yc.Filter{
				Field:    "name",
				Operator: yc.OperatorEq,
				Value:    instanceName,
			})
			if err != nil {
				log.Fatal(err)
			}
			if len(lst) == 0 {
				log.Fatalf("no compute instance found with name %s", instanceName)
			} else if len(lst) > 1 {
				log.Fatalf("found more than one compute instance with name %s", instanceName)
			}

			instanceId = lst[0].Id
		}
		if len(instanceName) == 0 {
			instanceName = instanceId
		}

		log.Infof("The compute instance with id %s will be %s ...", instanceId, action)
		var op *operation.Operation
		switch action {
		case "delete":
			op, err = client.ComputeInstanceDelete(ctx, instanceId)
		case "start":
			op, err = client.ComputeInstanceStart(ctx, instanceId)
		case "stop":
			op, err = client.ComputeInstanceStop(ctx, instanceId)
		default:
			log.Fatalf("unknown action %q", action)
		}
		if err != nil {
			log.Fatal(err)
		}

		if viper.GetBool("no-wait") {
			return
		}

		if err = op.Wait(ctx); err != nil {
			log.Fatal(err)
		}

		resp, err := op.Response()
		if err != nil {
			log.Fatal(err)
		}

		instance, ok := resp.(*computev1.Instance)
		if ok {
			ip := yc.GetIPv4(instance).PublicOrPrivate(true)
			log.Infof("The compute instance %s (%s) %s", instanceName, ip, action)
		}

		log.Infof("The compute instance %s %s", instanceName, action)
	}
}

func convert(in runtime.Object, out any, client *yc.Client) error {
	return scheme.Convert(in, out, scopes.CreateInstanceRequestConversionScope{
		CloudId:  client.CloudId(),
		FolderId: client.FolderId(),
		Sdk:      client.SDK(),
	})
}
