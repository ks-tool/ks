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
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/cmd/kube-scheduler/app/options"
)

// NewSchedulerCommand creates a *cobra.Command object with default parameters
func NewSchedulerCommand() *cobra.Command {
	opts := options.NewOptions()

	run := func(cmd *cobra.Command, args []string) error {
		setFlags(cmd)

		cc, sched, err := app.Setup(cmd.Context(), opts)
		if err != nil {
			return err
		}

		logger := klog.NewKlogr().WithName("scheduler")
		ctx := klog.NewContext(cmd.Context(), logger)

		err = app.Run(ctx, cc, sched)
		// https://github.com/kubernetes/kubernetes/blob/c9024e7ae628f1473a6cac28e7bd6cd8e64f872f/cmd/kube-scheduler/app/server.go#L316
		if err != nil && err.Error() == "finished without leader elect" {
			return nil
		}

		return err
	}

	return newCommand("kube-scheduler", *opts.Flags, run)
}
