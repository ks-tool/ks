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

const (
	ManagedKey = "managed"

	LabelClusterNameKey       = ""
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"

	UserDataKey      = "user-data"
	UserDataTemplate = `#cloud-config
users:
  - name: {{ .user }}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: {{ default "/bin/bash" .shell }}
    {{- with .sshAuthorizedKeys }}
    ssh_authorized_keys:
      {{- range $key := . }}
      - {{ $key }}
      {{- end }}
    {{- end }}
`
	UserDataK8sTemplate = UserDataTemplate + ``
)
