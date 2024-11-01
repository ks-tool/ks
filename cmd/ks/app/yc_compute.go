/*
Copyright © 2024 Alexey Shulutkov <github@shulutkov.ru>

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

package app

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/ks-tool/ks/pkg/common"
	"github.com/ks-tool/ks/pkg/yc"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ycComputeCmd represents the compute command
var ycComputeCmd = &cobra.Command{
	Aliases: []string{"vm"},
	Use:     "compute",
	Short:   "Manage Compute Cloud",
}

func init() {
	ycCmd.AddCommand(ycComputeCmd)

	computeCreateFlags(vmCreate)
	userdataFlags(vmCreate)
	noWait(vmDelete)
	noWait(vmStart)
	noWait(vmStop)
	vmListFlags(vmList)
	vmUserDataShowFlags(vmUserDataShow)
	userdataFlags(vmUserDataShow)

	ycComputeCmd.AddCommand(
		vmCreate,
		vmDelete,
		vmList,
		vmStart,
		vmStop,
		vmUserDataShow,
	)
}

var vmCreate = &cobra.Command{
	Use:   "create",
	Short: "Create a compute instance",
	PreRun: func(cmd *cobra.Command, args []string) {
		_ = viper.BindPFlags(cmd.Flags())
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if _, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && viper.IsSet(f.Name) {
				_ = cmd.Flags().Set(f.Name, viper.GetString(f.Name))
			}
		})
	},
	Run: func(cmd *cobra.Command, args []string) {
		var config *yc.ComputeInstanceConfig
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatal(err)
		}

		config.Labels = checkLabels(config.Labels)

		//resolve image-id
		client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}
		imageId, _ := client.ComputeImageGetId(cmd.Context(), "standard-images", config.ImageID)
		if len(imageId) == 0 {
			imageId, _ = client.ComputeImageGetId(cmd.Context(), config.FolderID, config.ImageID)
		}
		if len(imageId) > 0 {
			config.ImageID = imageId
		}

		var tpl string
		if len(config.UserDataFile) > 0 {
			file, err := homedir.Expand(config.UserDataFile)
			if err != nil {
				log.Fatal(err)
			}

			ud, err := os.ReadFile(file)
			if err != nil {
				log.Fatal(err)
			}
			tpl = string(ud)
		}

		if err := config.SetUserData(tpl); err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()

		op, err := client.ComputeInstanceCreate(ctx, config)
		if err != nil {
			log.Fatal(err)
		}

		meta, err := op.Metadata()
		if err != nil {
			log.Fatal(err)
		}

		newComputeInstance := meta.(*compute.CreateInstanceMetadata)
		log.Infof("Creating compute instance %s ...", newComputeInstance.InstanceId)

		if err = op.Wait(ctx); err != nil {
			log.Fatal(err)
		}

		resp, err := op.Response()
		if err != nil {
			log.Fatal(err)
		}

		instance := resp.(*compute.Instance)
		ip := yc.GetIPv4(instance).External()

		name := instance.Name
		if len(name) == 0 {
			name = instance.Id
		}

		log.Infof("The compute instance %q (%s) created", name, ip)
	},
}

var vmDelete = &cobra.Command{
	Aliases: []string{"rm", "del"},
	Use:     "delete <instance-id>",
	Short:   "Delete a compute instance",
	PreRunE: exactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Infof("The compute instance %s will be deleted", args[0])
		client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()

		op, err := client.ComputeInstanceDelete(ctx, args[0])
		if err != nil {
			log.Fatal(err)
		}

		if viper.GetBool("no-wait") {
			return
		}

		if err = op.Wait(ctx); err != nil {
			log.Fatal(err)
		}

		log.Info("The compute instance has been deleted")
	},
}

var vmList = &cobra.Command{
	Aliases: []string{"ls"},
	Use:     "list",
	Short:   "List of compute instances",
	PreRun: func(cmd *cobra.Command, args []string) {
		_ = viper.BindPFlags(cmd.Flags())
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()

		var lbl map[string]string
		if !viper.GetBool("all") {
			lbl = checkLabels(nil)
		}

		var filters []yc.Filter
		status := viper.GetString("status")
		if len(status) > 0 {
			if !yc.AllowStatus(status) {
				log.Fatalf("invalid status %q", status)
			}
			filters = append(filters, yc.Filter{
				Field:    "status",
				Operator: yc.OperatorEq,
				Value:    strings.ToUpper(status),
			})
		}

		folderId := viper.GetString("folder-id")
		lst, err := client.ComputeInstanceList(ctx, folderId, lbl, filters...)
		if err != nil {
			log.Fatal(err)
		}

		yc.FPrintComputeList(os.Stdout, lst)
	},
}

var vmStart = &cobra.Command{
	Use:     "start <instance-id>",
	Short:   "Start a compute instance",
	PreRunE: exactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()

		op, err := client.ComputeInstanceStart(ctx, args[0])
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

		instance := resp.(*compute.Instance)
		ip := yc.GetIPv4(instance).External()

		log.Infof("The compute instance %s (%s) started", instance.Name, ip)
	},
}

var vmStop = &cobra.Command{
	Use:     "stop <instance-id>",
	Short:   "Stop a compute instance",
	PreRunE: exactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()

		op, err := client.ComputeInstanceStop(ctx, args[0])
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

		instance := resp.(*compute.Instance)
		log.Infof("The compute instance %s stopped", instance.Name)
	},
}

var vmUserDataShow = &cobra.Command{
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

func computeCreateFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "a name of the instance")
	cmd.Flags().String("platform", yc.DefaultPlatformID, "specific platform for the instance")
	cmd.Flags().String("address", "", "assigns the given internal address to the instance that is created")
	cmd.Flags().Int64("cores", yc.DefaultCores, "specific number of CPU cores for an instance")
	cmd.Flags().Int64("core-fraction", yc.DefaultCoreFraction, "specific baseline performance for a core in percent")
	cmd.Flags().Int64("memory", yc.DefaultMemoryGib, "specific how much memory (in GiB) instance should have")
	cmd.Flags().String("disk-type", yc.DefaultDiskType, "the type of the disk")
	cmd.Flags().String("image-id", yc.DefaultImageID, "the source image used to create the disk")
	cmd.Flags().Int64("disk-size", yc.DefaultDiskSizeGib, "the size of the disk in GiB")
	cmd.Flags().Bool("preemptible", true, "creates preemptible instance")
	cmd.Flags().Bool("no-public-ip", false, "not use public IP for instance")
	cmd.Flags().String("sa", "", "service account name")
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
	_ = cmd.MarkFlagRequired("ssh-key")
}

func noWait(cmd *cobra.Command) {
	cmd.Flags().Bool("no-wait", false, "don't wait for completion")
}

func vmListFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "show all compute instances")
	cmd.Flags().String("status", "", "show compute instances with specific status. Allow: "+yc.Statuses())
}

func vmUserDataShowFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("template", false, "not render template")
}

func checkLabels(m map[string]string) map[string]string {
	if m == nil {
		m = make(map[string]string)
	}
	if v, ok := m[common.ManagedKey]; !ok || v != yc.KsToolKey {
		m[common.ManagedKey] = yc.KsToolKey
	}

	return m
}

// ExactArgs returns an error if there are not exactly n args.
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("accepts %d arg(s), received %d", n, len(args))
		}
		_ = viper.BindPFlags(cmd.Flags())
		return nil
	}
}
