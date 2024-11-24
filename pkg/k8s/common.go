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

package k8s

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilversion "k8s.io/apiserver/pkg/util/version"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
)

func init() {
	rest.SetDefaultWarningHandler(rest.NoWarnings{})
	_, featureGate := utilversion.DefaultComponentGlobalsRegistry.
		ComponentGlobalsOrRegister(
			utilversion.DefaultKubeComponent,
			utilversion.DefaultBuildEffectiveVersion(),
			utilfeature.DefaultMutableFeatureGate,
		)

	featureGate.AddMetrics()

	if err := logsapi.AddFeatureGates(featureGate); err != nil {
		klog.Fatal(err)
	}

	if err := utilversion.DefaultComponentGlobalsRegistry.Set(); err != nil {
		klog.Fatal(err)
	}
}

func setFlags(cmd *cobra.Command) {
	verflag.PrintAndExitIfRequested()
	cliflag.PrintFlags(cmd.Flags())
}

func newCommand(
	name string,
	namedFlagSets cliflag.NamedFlagSets,
	run func(cmd *cobra.Command, args []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:          name,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := run(cmd, args)
			if errors.Is(err, context.Canceled) {
				return nil
			}

			return err
		},
	}

	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name(), logs.SkipLoggingConfigurationFlags())
	for _, f := range namedFlagSets.FlagSets {
		cmd.Flags().AddFlagSet(f)
	}

	return cmd
}
