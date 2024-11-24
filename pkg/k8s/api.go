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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	"k8s.io/kubernetes/cmd/kube-apiserver/app/options"
)

// NewAPIServerCommand creates a *cobra.Command object with default parameters
func NewAPIServerCommand() *cobra.Command {
	opts := options.NewServerRunOptions()

	run := func(cmd *cobra.Command, args []string) error {
		setFlags(cmd)

		completedOptions, err := opts.Complete()
		if err != nil {
			return err
		}

		if errs := completedOptions.Validate(); len(errs) != 0 {
			return utilerrors.NewAggregate(errs)
		}

		return app.Run(cmd.Context(), completedOptions)
	}

	return newCommand("kube-apiserver", opts.Flags(), run)
}
