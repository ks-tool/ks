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
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
)

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand() *cobra.Command {
	opts, err := options.NewKubeControllerManagerOptions()
	if err != nil {
		klog.Background().Error(err, "Unable to initialize command options")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	run := func(cmd *cobra.Command, args []string) error {
		setFlags(cmd)

		c, err := opts.Config(app.KnownControllers(), app.ControllersDisabledByDefault(), app.ControllerAliases())
		if err != nil {
			return err
		}

		logger := klog.NewKlogr().WithName("controller-manager")
		ctx := klog.NewContext(cmd.Context(), logger)

		return app.Run(ctx, c.Complete())
	}

	namedFlagSets := opts.Flags(app.KnownControllers(), app.ControllersDisabledByDefault(), app.ControllerAliases())
	return newCommand("kube-apiserver", namedFlagSets, run)
}
