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
	computev1 "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1/instancegroup"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(addKnownTypes)
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ComputeInstance{},
		&Kubernetes{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

type ComputeInstanceSpec struct {
	// +optional
	Platform string `json:"platform,omitempty" mapstructure:"platform"`
	// +optional
	Preemptible bool `json:"preemptible,omitempty" mapstructure:"preemptible"`
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty" mapstructure:"sa"`
	// +optional
	UserData string `json:"userData,omitempty"`
	// +optional
	Resources ResourcesSpec `json:"resources,omitempty"`
	// +optional
	Disk DiskSpec `json:"disk,omitempty" mapstructure:"disk"`
	// +optional
	Folder string `json:"folder,omitempty" mapstructure:"folder"`
	// +optional
	Zone string `json:"zone,omitempty" mapstructure:"zone"`
	// +optional
	NetworkInterfaces []NetworkInterfaceSpec `json:"networkInterfaces"`
}

type ResourcesSpec struct {
	// +optional
	Memory int `json:"memory,omitempty" mapstructure:"memory"`
	// +optional
	Cpu int `json:"cpu,omitempty" mapstructure:"cpu"`
	// +optional
	CoreFraction int `json:"coreFraction,omitempty" mapstructure:"core-fraction"`
}

type DiskSpec struct {
	// +optional
	Size int `json:"size,omitempty" mapstructure:"size"`
	// +optional
	Type string `json:"type,omitempty" mapstructure:"type"`
	// +optional
	Image string `json:"image,omitempty" mapstructure:"image"`
}

type NetworkInterfaceSpec struct {
	// +optional
	Subnet string `json:"subnet"`
	// +optional
	PublicIp *string `json:"publicIp,omitempty"`
	// +optional
	PrivateIp string `json:"privateIp,omitempty"`
	// +optional
	SecurityGroups []string `json:"securityGroups,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComputeInstance is the Schema for the computeinstance
type ComputeInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ComputeInstanceSpec `json:"spec,omitempty"`
}

type KubernetesSpec struct {
	ControlPlain ComputeInstanceSpec   `json:"controlPlain"`
	NodeGroups   []ComputeInstanceSpec `json:"nodeGroups"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Kubernetes is the Schema for the kubernetes
type Kubernetes struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KubernetesSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen=false

// KubernetesRequest is the Schema for the kubernetesrequest
type KubernetesRequest struct {
	ControlPlain *computev1.CreateInstanceRequest
	NodeGroups   []*instancegroup.CreateInstanceGroupRequest
}

// +k8s:deepcopy-gen=false

type ComputeInstanceList struct{}

// +k8s:deepcopy-gen=false

type KubernetesList struct{}
