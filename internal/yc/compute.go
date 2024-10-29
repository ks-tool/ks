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

package yc

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
	"github.com/spf13/viper"
)

// Compute represents the compute command
func Compute() *cobra.Command {
	cmd := &cobra.Command{
		Aliases: []string{"vm"},
		Use:     "compute",
		Short:   "A brief description of your command",
	}

	computeCreateFlags(vmCreate)
	noWait(vmDelete)
	noWait(vmStart)
	noWait(vmStop)
	vmListFlags(vmList)
	vmUserDataShowFlags(vmUserDataShow)

	cmd.AddCommand(
		vmCreate,
		vmDelete,
		vmList,
		vmStart,
		vmStop,
		vmUserDataShow,
	)

	return cmd
}

var vmCreate = &cobra.Command{
	Use:   "create",
	Short: "Create a compute instance",
	PreRun: func(cmd *cobra.Command, args []string) {
		_ = viper.BindPFlags(cmd.Flags())
	},
	Run: func(cmd *cobra.Command, args []string) {
		var config *yc.ComputeInstanceConfig
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatal(err)
		}

		config.Labels = checkLabels(config.Labels)

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

		client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
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

		log.Infof("The compute instance %s (%s) created", instance.Name, ip)
	},
}

var vmDelete = &cobra.Command{
	Aliases: []string{"rm", "del"},
	Use:     "delete <instance-id>",
	Short:   "Delete a compute instance",
	PreRun: func(cmd *cobra.Command, args []string) {
		_ = viper.BindPFlags(cmd.Flags())
	},
	PreRunE: cobra.ExactArgs(1),
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
			log.Info("The compute instance will be deleted async ...")
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
	Use:   "start <instance-id>",
	Short: "Start a compute instance",
	PreRun: func(cmd *cobra.Command, args []string) {
		_ = viper.BindPFlags(cmd.Flags())
	},
	PreRunE: cobra.ExactArgs(1),
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
			log.Info("The compute instance will be deleted async ...")
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
	Use:   "stop <instance-id>",
	Short: "Stop a compute instance",
	PreRun: func(cmd *cobra.Command, args []string) {
		_ = viper.BindPFlags(cmd.Flags())
	},
	PreRunE: cobra.ExactArgs(1),
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
			log.Info("The compute instance will be deleted async ...")
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
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("platform-id", yc.DefaultPlatformID, "")
	cmd.Flags().String("address", "", "")
	cmd.Flags().Int64("cores", yc.DefaultCores, "")
	cmd.Flags().Int64("core-fraction", yc.DefaultCoreFraction, "")
	cmd.Flags().Int64("memory", yc.DefaultMemoryGib, "")
	cmd.Flags().String("disk-type", yc.DefaultDiskType, "")
	cmd.Flags().String("disk-id", yc.DefaultDiskID, "set boot disk id")
	cmd.Flags().Int64("disk-size", yc.DefaultDiskSizeGib, "")
	cmd.Flags().Bool("preemptible", true, "")
	cmd.Flags().Bool("no-public-ip", false, "")
	cmd.Flags().String("sa", "", "service account name")
	cmd.Flags().String("user-data-file", "", "")

	cmd.Flags().StringSlice("ssh-pub", nil, "")
	_ = cmd.MarkFlagRequired("ssh-pub")

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	cmd.Flags().String("user", usr.Username, "")
	_ = cmd.MarkFlagRequired("user")

	cmd.Flags().String("shell", "/bin/bash", "set login shell for user")
}

func noWait(cmd *cobra.Command) {
	cmd.Flags().Bool("no-wait", false, "don't wait for completion")
}

func vmListFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "show all compute instances")
	cmd.Flags().String("status", "", "show compute instances with specific status. Allow: "+yc.Statuses())
}

func vmUserDataShowFlags(cmd *cobra.Command) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Flags().String("user", usr.Username, "")
	cmd.Flags().StringSlice("ssh-pub", nil, "")
	cmd.Flags().String("shell", "/bin/bash", "set login shell for user")
	cmd.Flags().String("user-data-file", "", "")
	cmd.Flags().Bool("template", false, "show template")
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
