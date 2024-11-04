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

package common

import (
	gouser "os/user"

	"github.com/ks-tool/ks/pkg/utils"
)

const (
	ManagedKey = "managed"
	KsToolKey  = "yc.ks-tool.dev"

	DefaultCores        = 2
	DefaultDiskSizeGib  = 10
	DefaultCoreFraction = 100
	DefaultMemoryGib    = 2

	LabelClusterNameKey       = ""
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"

	UserDataKey      = "user-data"
	UserDataTemplate = `#cloud-config
users:
  - name: {{ .user }}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: {{ default "/bin/bash" .shell }}
    {{- with .sshKeys }}
    ssh_authorized_keys:
      {{- range $key := . }}
      - {{ $key }}
      {{- end }}
    {{- end }}
`
	UserDataK8sTemplate = UserDataTemplate + ``
)

func DefaultUserData(user string, keys ...string) (string, error) {
	usr, err := gouser.Current()
	if err != nil {
		return "", err
	}

	if len(user) == 0 {
		user = usr.Username
	}

	if len(keys) == 0 {
		keys, err = utils.GetAllSshPublicKeys(usr.HomeDir)
		if err != nil {
			return "", err
		}
	}

	return utils.Template(UserDataTemplate, map[string]any{
		"user":    user,
		"sshKeys": keys,
	})
}
