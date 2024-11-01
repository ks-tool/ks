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
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
)

func FPrintComputeList(w io.Writer, lst []*compute.Instance) {
	tbl := table.NewWriter()
	tbl.SetOutputMirror(w)
	tbl.AppendHeader(table.Row{"ID", "Name", "IP", "Status", "Zone", "SubnetID", "Platform"})

	for _, item := range lst {
		tbl.AppendRow(table.Row{
			item.Id,
			item.Name,
			GetIPv4(item).External(),
			item.Status.String(),
			item.ZoneId,
			item.NetworkInterfaces[0].SubnetId,
			item.PlatformId,
		})
	}

	tbl.Render()
}

func FPrintClusterGet(w io.Writer, lst []*compute.Instance) {
	tbl := table.NewWriter()
	tbl.SetOutputMirror(w)
	tbl.AppendHeader(table.Row{"ComputeID", "Name", "IP", "Status", "Zone", "SubnetID", "Platform"})

	for _, item := range lst {
		tbl.AppendRow(table.Row{
			item.Id,
			item.Name,
			item.NetworkInterfaces[0].PrimaryV4Address.OneToOneNat.Address,
			item.Status.String(),
			item.ZoneId,
			item.NetworkInterfaces[0].SubnetId,
			item.PlatformId,
		})
	}

	tbl.Render()
}
