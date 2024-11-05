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
	"github.com/yandex-cloud/go-sdk/sdkresolvers"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
			bind(cmd, "platform")
			bind(cmd, "sa")
			bind(cmd, "preemptible")
			bind(cmd, "address")
			bind(cmd, "cpu", "resources.cpu")
			bind(cmd, "core-fraction", "resources.core-fraction")
			bind(cmd, "memory", "resources.memory")
			bind(cmd, "disk-type", "disk.type")
			bind(cmd, "disk-image", "disk.image")
			bind(cmd, "disk-size", "disk.size")
			bind(cmd, "no-public-ip")
			bind(cmd, "subnet")
			bind(cmd, "manifest")
			bind(cmd, "user")
			bind(cmd, "shell")
			bind(cmd, "user-data-file")
			bind(cmd, "ssh-key")
		},
		Run: func(cmd *cobra.Command, args []string) {
			var opts []yc.ClientConfig
			cloudName := viper.GetString("cloud")
			if viper.IsSet("cloud") {
				opts = append(opts, yc.CloudName(cloudName))
			}

			folderName := viper.GetString("folder")
			if viper.IsSet("folder") {
				opts = append(opts, yc.FolderName(folderName))
			}

			client, err := yc.NewClient(viper.GetString("token"), opts...)
			if err != nil {
				log.Fatal(err)
			}

			var obj runtime.Object
			if viper.IsSet("manifest") {
				obj, err = scheme.FromFileWithDefaults(viper.GetString("manifest"))
				if err != nil {
					log.Fatal(err)
				}
			} else {
				obj, err = computeInstanceFromFlags(client)
				if err != nil {
					log.Fatal(err)
				}
			}

			if len(args) > 0 {
				obj.(*ycv1alpha1.ComputeInstance).Name = args[0]
			}

			req := &computev1.CreateInstanceRequest{}
			if err = scheme.Convert(obj, req, scopes.CreateInstanceRequestConversionScope{
				CloudId:  client.CloudId(),
				FolderId: client.FolderId(),
				Sdk:      client.SDK(),
			}); err != nil {
				log.Fatal(err)
			}

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

			name := instance.Name
			if len(name) == 0 {
				name = instance.Id
			}

			log.Infof("The compute instance %q (%s) created", name, yc.GetIPv4(instance).PublicOrPrivate())
		},
	}

	vmDelete = &cobra.Command{
		Aliases: []string{"rm", "del"},
		Use:     "delete",
		Short:   "Delete a compute instance",
		PreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.Infof("The compute instance %s will be deleted ...", args[0])

			var opts []yc.ClientConfig
			cloudName := viper.GetString("cloud")
			if len(cloudName) > 0 {
				opts = append(opts, yc.CloudName(cloudName))
			}

			folderName := viper.GetString("folder")
			if len(folderName) > 0 {
				opts = append(opts, yc.FolderName(folderName))
			}

			client, err := yc.NewClient(viper.GetString("token"), opts...)
			if err != nil {
				log.Fatal(err)
			}

			op, err := client.ComputeInstanceDelete(cmd.Context(), args[0])
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}

			log.Info("The compute instance has been deleted")
		},
	}

	vmList = &cobra.Command{
		Aliases: []string{"ls"},
		Use:     "list",
		Short:   "List of compute instances",
		PreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			var opts []yc.ClientConfig
			cloudName := viper.GetString("cloud")
			if len(cloudName) > 0 {
				opts = append(opts, yc.CloudName(cloudName))
			}

			folderName := viper.GetString("folder")
			if len(folderName) > 0 {
				opts = append(opts, yc.FolderName(folderName))
			}

			client, err := yc.NewClient(viper.GetString("token"), opts...)
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

			var lbl map[string]string
			if !viper.GetBool("all") {
				lbl = map[string]string{common.ManagedKey: common.KsToolKey}
			}
			lst, err := client.ComputeInstanceList(cmd.Context(), "", lbl, filters...)
			if err != nil {
				log.Fatal(err)
			}

			yc.FPrintComputeList(os.Stdout, lst)
		},
	}

	vmStart = &cobra.Command{
		Use:     "start <instance-id>",
		Short:   "Start a compute instance",
		PreRunE: exactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var opts []yc.ClientConfig
			cloudName := viper.GetString("cloud")
			if len(cloudName) > 0 {
				opts = append(opts, yc.CloudName(cloudName))
			}

			folderName := viper.GetString("folder")
			if len(folderName) > 0 {
				opts = append(opts, yc.FolderName(folderName))
			}

			client, err := yc.NewClient(viper.GetString("token"), opts...)
			if err != nil {
				log.Fatal(err)
			}

			op, err := client.ComputeInstanceStart(cmd.Context(), args[0])
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}

			resp, err := op.Response()
			if err != nil {
				log.Fatal(err)
			}

			instance := resp.(*computev1.Instance)
			log.Infof("The compute instance %s (%s) started",
				instance.Name, yc.GetIPv4(instance).PublicOrPrivate())
		},
	}

	vmStop = &cobra.Command{
		Use:     "stop <instance-id>",
		Short:   "Stop a compute instance",
		PreRunE: exactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var opts []yc.ClientConfig
			cloudName := viper.GetString("cloud")
			if len(cloudName) > 0 {
				opts = append(opts, yc.CloudName(cloudName))
			}

			folderName := viper.GetString("folder")
			if len(folderName) > 0 {
				opts = append(opts, yc.FolderName(folderName))
			}

			client, err := yc.NewClient(viper.GetString("token"), opts...)
			if err != nil {
				log.Fatal(err)
			}

			op, err := client.ComputeInstanceStop(cmd.Context(), args[0])
			if err != nil {
				log.Fatal(err)
			}

			if viper.GetBool("no-wait") {
				return
			}

			if err = op.Wait(cmd.Context()); err != nil {
				log.Fatal(err)
			}

			resp, err := op.Response()
			if err != nil {
				log.Fatal(err)
			}

			instance := resp.(*computev1.Instance)
			log.Infof("The compute instance %s stopped", instance.Name)
		},
	}

	vmUserDataShow = &cobra.Command{
		Aliases: []string{"ud"},
		Use:     "user-data",
		Short:   "Show user-data for a compute instance",
		PreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			var config *yc.ComputeInstanceConfig
			if err := viper.Unmarshal(&config); err != nil {
				log.Fatal(err)
			}

			tpl := common.UserDataTemplate
			if len(config.UserDataFile) > 0 {
				file, err := homedir.Expand(config.UserDataFile)
				if err != nil {
					log.Fatal(err)
				}
				b, err := os.ReadFile(file)
				if err != nil {
					log.Fatal(err)
				}
				tpl = string(b)
			}
			if viper.GetBool("template") {
				fmt.Print(tpl)
				return
			}

			if err := config.SetUserData(tpl); err != nil {
				log.Fatal(err)
			}

			fmt.Print(config.Metadata[common.UserDataKey])
		},
	}
)

func init() {
	computeCreateFlags(vmCreate)
	userdataFlags(vmCreate)
	deleteFlags(vmDelete)
	noWait(vmDelete)
	noWait(vmStart)
	noWait(vmStop)
	vmListFlags(vmList)
	vmUserDataShowFlags(vmUserDataShow)
	userdataFlags(vmUserDataShow)

	computeCmd.AddCommand(
		vmCreate,
		vmDelete,
		vmList,
		vmStart,
		vmStop,
		vmUserDataShow,
	)

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
	cmd.Flags().Bool("no-public-ip", false, "not use public IP for instance")
	cmd.Flags().String("subnet", "", "specific the name of the subnet")
	cmd.Flags().String("manifest", "", "path to manifest file")
}

func userdataFlags(cmd *cobra.Command) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Flags().String("user", usr.Username, "create user with specific name")
	cmd.Flags().String("shell", "/bin/bash", "set login shell for user")
	cmd.Flags().String("user-data-file", "", "custom user-data file")
	cmd.Flags().StringSlice("ssh-key", nil, "add public SSH key from specified file to authorized_keys")
}

func noWait(cmd *cobra.Command) {
	cmd.Flags().Bool("no-wait", false, "don't wait for completion")
}

func deleteFlags(cmd *cobra.Command) {
	cmd.Flags().String("id", "", "id of compute instance")
	cmd.Flags().String("name", "", "name of compute instance")
	cmd.MarkFlagsMutuallyExclusive("id", "name")
}

func vmListFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "show all compute instances")
	cmd.Flags().String("status", "", "show compute instances with specific status. Allow: "+yc.Statuses())
}

func vmUserDataShowFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("template", false, "not render template")
}

func bind(cmd *cobra.Command, name string, key ...string) {
	viperKey := name
	if len(key) > 0 {
		viperKey = key[0]
	}
	_ = viper.BindPFlag(viperKey, cmd.Flags().Lookup(name))
}

// exactArgs returns an error if there are not exactly n args.
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("accepts %d arg(s), received %d", n, len(args))
		}
		_ = viper.BindPFlags(cmd.Flags())
		return nil
	}
}

func computeInstanceFromFlags(client *yc.Client) (*ycv1alpha1.ComputeInstance, error) {
	obj := new(ycv1alpha1.ComputeInstance)
	if err := viper.Unmarshal(&obj.Spec); err != nil {
		return nil, err
	}

	userDataTemplate := common.UserDataTemplate
	if viper.IsSet("user-data-file") {
		file, err := homedir.Expand(viper.GetString("user-data-file"))
		if err != nil {
			return nil, err
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		userDataTemplate = string(data)
	}
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	var keys []string
	if viper.IsSet("ssh-key") {
		for _, keyFile := range viper.GetStringSlice("ssh-key") {
			keyFile, err = homedir.Expand(keyFile)
			if err != nil {
				return nil, err
			}
			b, err := os.ReadFile(keyFile)
			if err != nil {
				return nil, err
			}
			keys = append(keys, string(b))
		}
	} else {
		keys, err = utils.GetAllSshPublicKeys(usr.HomeDir)
		if err != nil {
			return nil, err
		}
	}

	obj.Spec.UserData, err = utils.Template(userDataTemplate, map[string]any{
		"user":    viper.GetString("user"),
		"sshKeys": keys,
		"shell":   viper.GetString("shell"),
	})
	if err != nil {
		return nil, err
	}

	if viper.IsSet("subnet") {
		subnet := sdkresolvers.SubnetResolver(viper.GetString("subnet"), sdkresolvers.FolderID(client.FolderId()))
		if err = subnet.Run(context.Background(), client.SDK()); err != nil {
			return nil, err
		}
		if subnet.Err() != nil {
			return nil, subnet.Err()
		}

		netif := ycv1alpha1.NetworkInterfaceSpec{
			Subnet:    subnet.ID(),
			PrivateIp: viper.GetString("address"),
		}
		if viper.GetBool("no-public-ip") {
			pub := ""
			netif.PublicIp = &pub
		}

		obj.Spec.NetworkInterfaces = append(obj.Spec.NetworkInterfaces, netif)
	}

	scheme.Defaults(obj)

	return obj, nil
}
