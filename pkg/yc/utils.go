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

package yc

import computev1 "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"

type ComputeInstanceIPv4 struct {
	e, i string
}

func (i ComputeInstanceIPv4) PublicOrPrivate() string {
	if len(i.e) == 0 {
		return i.i
	}

	return i.e
}

func (i ComputeInstanceIPv4) Private() string {
	return i.i
}

func GetIPv4(i *computev1.Instance) ComputeInstanceIPv4 {
	ipv4 := i.NetworkInterfaces[0].PrimaryV4Address
	var e string
	if ipv4.OneToOneNat != nil {
		e = ipv4.OneToOneNat.Address
	}

	return ComputeInstanceIPv4{
		e: e,
		i: ipv4.Address,
	}
}
