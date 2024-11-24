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

package v1alpha1

import (
	"github.com/ks-tool/ks/pkg/common"
)

const (
	DefaultDiskType = "network-ssd"
	DefaultImageID  = "debian-12-v20240920"

	DefaultPlatformID = "standard-v3"
	DefaultZone       = "ru-central1-b"
)

func init() {
	localSchemeBuilder.Register(RegisterDefaults)
}

// SetDefaults_ComputeInstance sets default values for ComputeInstance objects.
func SetDefaults_ComputeInstance(obj *ComputeInstance) {
	if len(obj.Spec.Zone) == 0 {
		obj.Spec.Zone = DefaultZone
	}
	if len(obj.Spec.Platform) == 0 {
		obj.Spec.Platform = DefaultPlatformID
	}
	if len(obj.Spec.Disk.Type) == 0 {
		obj.Spec.Disk.Type = DefaultDiskType
	}
	if obj.Spec.Disk.Size == 0 {
		obj.Spec.Disk.Size = common.DefaultDiskSizeGib
	}
	if len(obj.Spec.Disk.Image) == 0 {
		obj.Spec.Disk.Image = DefaultImageID
	}
	if obj.Spec.Resources.Cpu == 0 {
		obj.Spec.Resources.Cpu = common.DefaultCores
	}
	if obj.Spec.Resources.CoreFraction == 0 {
		obj.Spec.Resources.CoreFraction = common.DefaultCoreFraction
	}
	if obj.Spec.Resources.Memory == 0 {
		obj.Spec.Resources.Memory = common.DefaultMemoryGib
	}
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}

	obj.Labels[common.ManagedKey] = common.KsToolKey
}
