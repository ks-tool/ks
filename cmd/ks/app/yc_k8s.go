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
	"os"

	"github.com/ks-tool/ks/pkg/common"
	"github.com/ks-tool/ks/pkg/yc"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ycK8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Manage scratch Kubernetes clusters",
}

// K8s represents the k8s command
func init() {
	ycCmd.AddCommand(ycK8sCmd)

	computeCreateFlags(clusterCreate)
	noWait(clusterCreate)
	noWait(clusterDelete)
	noWait(clusterStart)
	noWait(clusterStop)
	clusterScaleFlags(clusterScale)
	noWait(clusterScale)

	ycK8sCmd.AddCommand(
		clusterCreate,
		clusterDelete,
		clusterGet,
		clusterList,
		clusterStart,
		clusterStop,
		clusterScale,
	)
}

var clusterCreate = &cobra.Command{
	Use:   "create",
	Short: "Create a Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		var config *yc.ComputeInstanceConfig
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatal(err)
		}

		var tpl string
		if len(config.UserDataFile) > 0 {
			ud, err := os.ReadFile(config.UserDataFile)
			if err != nil {
				log.Fatal(err)
			}
			tpl = string(ud)
		} else {
			tpl = common.UserDataK8sTemplate
		}

		if err := config.SetUserData(tpl); err != nil {
			log.Fatal(err)
		}

		config.Labels = map[string]string{
			common.LabelNodeRoleControlPlane: "",
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
		log.Infof("Creating Kubernetes cluster %s ...", newComputeInstance.InstanceId)

		if err = op.Wait(ctx); err != nil {
			log.Fatal(err)
		}

		resp, err := op.Response()
		if err != nil {
			log.Fatal(err)
		}

		instance := resp.(*compute.Instance)
		ip := yc.GetIPv4(instance).External()

		log.Infof("The Kubernetes cluster %s (%s) created", instance.Name, ip)
	},
}

var clusterDelete = &cobra.Command{
	Use:   "delete <cluster-id>",
	Short: "Delete a Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		/*client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()

		folderId := viper.GetString("folder-id")
		client.ComputeInstanceList(ctx, folderId, map[string]string{""})*/
	},
}

var clusterGet = &cobra.Command{
	Use:   "get [cluster-id, ...]",
	Short: "Get a Kubernetes cluster info",
	Run: func(cmd *cobra.Command, args []string) {
		/*client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()*/
	},
}

var clusterList = &cobra.Command{
	Use:   "list",
	Short: "List a Kubernetes clusters",
	Run: func(cmd *cobra.Command, args []string) {
		/*client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()

		lst, err := client.ComputeInstanceList(ctx, args[0], map[string]string{""})
		if err != nil {
			log.Fatal(err)
		}

		yc.FPrintComputeList(os.Stdout, lst)*/
	},
}

var clusterStart = &cobra.Command{
	Use:   "start <cluster-id>",
	Short: "Start a Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		/*client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()*/
	},
}

var clusterStop = &cobra.Command{
	Use:   "stop <cluster-id>",
	Short: "Stop a Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		/*client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()*/
	},
}

var clusterScale = &cobra.Command{
	Use:   "scale <cluster-id>",
	Short: "Scale Kubernetes workers",
	Run: func(cmd *cobra.Command, args []string) {
		/*client, err := yc.NewClient(viper.GetString("token"))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
		defer cancel()*/
	},
}

func clusterScaleFlags(cmd *cobra.Command) {
	cmd.Flags().Uint("replicas", 0, "number of worker node replicas")
	cmd.Flags().StringSlice("delete-node", nil, "scale down and delete specific node")
}
