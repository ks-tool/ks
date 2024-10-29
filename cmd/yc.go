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

package cmd

import (
	"os"
	"time"

	YC "github.com/ks-tool/ks/internal/yc"
	"github.com/ks-tool/ks/pkg/yc"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ycCmd represents the yc command
var ycCmd = &cobra.Command{
	Use:   "yc",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

func init() {
	rootCmd.AddCommand(ycCmd)
	cobra.OnInitialize(setTokenFromViper, setAllValueFlagsFromViper(ycCmd))

	ycCmd.AddCommand(YC.Compute(), YC.K8s())

	ycCmd.PersistentFlags().StringP("folder-id", "f", "", "")
	_ = ycCmd.MarkPersistentFlagRequired("folder-id")

	ycCmd.PersistentFlags().StringP("subnet-id", "s", "", "")
	ycCmd.PersistentFlags().StringP("zone", "z", yc.DefaultZone, "")
	ycCmd.PersistentFlags().DurationP("timeout", "t", 180*time.Second, "")
	ycCmd.PersistentFlags().StringP("token-file", "k", "", "")
	ycCmd.PersistentFlags().String("token", "", "Env variable: YC_TOKEN")
	ycCmd.MarkFlagsMutuallyExclusive("token", "token-file")
	_ = ycCmd.MarkPersistentFlagRequired("token")

	if err := viper.BindEnv("token", "YC_TOKEN"); err != nil {
		log.Fatal(err)
	}

	_ = viper.BindPFlags(ycCmd.PersistentFlags())
}

func setTokenFromViper() {
	token := viper.GetString("token")
	if len(token) > 0 {
		return
	}

	tokenFile := viper.GetString("token-file")
	if len(tokenFile) > 0 {
		tok, err := os.ReadFile(tokenFile)
		if err != nil {
			log.Fatalf("read token file failed: %s", err)
		}
		_ = ycCmd.Flags().Set("token", string(tok))
	}
}

func setAllValueFlagsFromViper(cmd *cobra.Command) func() {
	return func() {
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if _, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && viper.IsSet(f.Name) {
				_ = cmd.PersistentFlags().Set(f.Name, viper.GetString(f.Name))
			}
		})
	}
}
