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
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ks",
	Short: "A brief description of your application",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringP("config", "c", "config", "config file (search in $HOME/.ks directory)")
	rootCmd.PersistentFlags().Bool("debug", false, "enable debug mode")

	_ = viper.BindPFlags(rootCmd.PersistentFlags())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ks")
	viper.SetConfigType("yaml")

	// Find home directory.
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	viper.AddConfigPath(filepath.Join(home, ".ks"))
	viper.SetConfigName(viper.GetString("config"))

	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	// If a config file is found, read it in.
	if err = viper.ReadInConfig(); err == nil {
		log.Debugf("Using config file: %s", viper.ConfigFileUsed())
	}
}
