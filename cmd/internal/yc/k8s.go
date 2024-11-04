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
	"github.com/spf13/cobra"
)

var (
	k8sCmd = &cobra.Command{
		Use:   "k8s",
		Short: "Manage scratch Kubernetes clusters",
	}

	k8sClusterCreate = &cobra.Command{
		Use:   "create",
		Short: "Create a Kubernetes cluster",
		Run: func(cmd *cobra.Command, args []string) {
			//var config *yc.ComputeInstanceConfig
			//if err := viper.Unmarshal(&config); err != nil {
			//	log.Fatal(err)
			//}
			//
			//var tpl string
			//if len(config.UserDataFile) > 0 {
			//	ud, err := os.ReadFile(config.UserDataFile)
			//	if err != nil {
			//		log.Fatal(err)
			//	}
			//	tpl = string(ud)
			//} else {
			//	tpl = common.UserDataK8sTemplate
			//}
			//
			//if err := config.SetUserData(tpl); err != nil {
			//	log.Fatal(err)
			//}
			//
			//config.Labels = map[string]string{
			//	common.LabelNodeRoleControlPlane: "",
			//}
			//
			//client, err := yc.NewClient(viper.GetString("token"))
			//if err != nil {
			//	log.Fatal(err)
			//}
			//
			//ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("timeout"))
			//defer cancel()
			//
			//op, err := client.ComputeInstanceCreate(ctx, config)
			//if err != nil {
			//	log.Fatal(err)
			//}
			//
			//meta, err := op.Metadata()
			//if err != nil {
			//	log.Fatal(err)
			//}
			//newComputeInstance := meta.(*compute.CreateInstanceMetadata)
			//log.Infof("Creating Kubernetes cluster %s ...", newComputeInstance.InstanceId)
			//
			//if err = op.Wait(ctx); err != nil {
			//	log.Fatal(err)
			//}
			//
			//resp, err := op.Response()
			//if err != nil {
			//	log.Fatal(err)
			//}
			//
			//instance := resp.(*compute.Instance)
			//ip := yc.GetIPv4(instance).External()
			//
			//log.Infof("The Kubernetes cluster %s (%s) created", instance.Name, ip)
		},
	}

	k8sClusterDelete = &cobra.Command{
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

	k8sClusterGet = &cobra.Command{
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

	k8sClusterList = &cobra.Command{
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

	k8sClusterStart = &cobra.Command{
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

	k8sClusterStop = &cobra.Command{
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

	k8sClusterScale = &cobra.Command{
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
)

// K8s represents the k8s command
func init() {
	computeCreateFlags(k8sClusterCreate)
	noWait(k8sClusterCreate)
	noWait(k8sClusterDelete)
	noWait(k8sClusterStart)
	noWait(k8sClusterStop)
	clusterScaleFlags(k8sClusterScale)
	noWait(k8sClusterScale)

	k8sCmd.AddCommand(
		k8sClusterCreate,
		k8sClusterDelete,
		k8sClusterGet,
		k8sClusterList,
		k8sClusterStart,
		k8sClusterStop,
		k8sClusterScale,
	)
}

func clusterScaleFlags(cmd *cobra.Command) {
	cmd.Flags().Uint("replicas", 0, "number of worker node replicas")
	cmd.Flags().StringSlice("delete-node", nil, "scale down and delete specific node")
}
