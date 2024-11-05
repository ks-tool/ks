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
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var YandexCloudCmd = &cobra.Command{
	Use:   "yc",
	Short: "Manage Yandex Cloud resources",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if _, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && viper.IsSet(f.Name) {
				_ = cmd.Flags().Set(f.Name, viper.GetString(f.Name))
			}
		})

		if viper.IsSet("token-file") {
			tokenFilePath, err := homedir.Expand(viper.GetString("token-file"))
			if err != nil {
				log.Fatal(err)
			}

			tokenBytes, err := os.ReadFile(tokenFilePath)
			if err != nil {
				log.Fatal(err)
			}
			viper.Set("token", strings.TrimSpace(string(tokenBytes)))
		}
	},
}

func init() {
	YandexCloudCmd.AddCommand(computeCmd, k8sCmd)

	YandexCloudCmd.PersistentFlags().String("cloud", "", "set the name of the cloud to use")
	YandexCloudCmd.PersistentFlags().String("folder", "", "set the name of the folder to use")
	YandexCloudCmd.PersistentFlags().String("token-file", "", "read token from file")
	YandexCloudCmd.PersistentFlags().String("token", "", "set token for Yandex Cloud interact. Can use env variable YC_TOKEN")
	YandexCloudCmd.MarkFlagsMutuallyExclusive("token", "token-file")
	_ = viper.BindPFlags(YandexCloudCmd.PersistentFlags())

	_ = YandexCloudCmd.MarkPersistentFlagRequired("token")
	_ = viper.BindEnv("token", "YC_TOKEN")
}
