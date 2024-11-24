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

import (
	"fmt"
	"strings"

	computev1 "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
)

type ComputeInstanceIPv4 struct {
	e, i []string
}

func (i ComputeInstanceIPv4) PublicOrPrivate(inLine bool) string {
	n := len(i.i)
	if len(i.e) > n {
		n = len(i.e)
	}

	appendStr := func(s string, idx int) string {
		if n == 1 {
			return fmt.Sprintf("%s", s)
		}

		return fmt.Sprintf("%s (%d)", s, idx)
	}

	out := make([]string, 0)
	for k := 0; k < n; k++ {
		if len(i.e) > 0 && k < len(i.e) {
			out = append(out, appendStr(i.e[k], k))
		}
		if k < len(i.i) {
			out = append(out, appendStr(i.i[k], k))
		}
	}

	sep := "\n"
	if inLine {
		sep = ", "
	}

	return strings.Join(out, sep)
}

func (i ComputeInstanceIPv4) Public() []string {
	return i.e
}

func (i ComputeInstanceIPv4) Private() []string {
	return i.i
}

func GetIPv4(i *computev1.Instance) ComputeInstanceIPv4 {
	ips := ComputeInstanceIPv4{}
	for _, netif := range i.NetworkInterfaces {
		if netif.PrimaryV4Address.OneToOneNat != nil {
			ips.e = append(ips.e, netif.PrimaryV4Address.OneToOneNat.Address)
		}
		ips.i = append(ips.i, netif.PrimaryV4Address.Address)
	}

	return ips
}
