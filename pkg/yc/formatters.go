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

var StyleK8s = table.Style{
	Name: "StyleSimple",
	Box: table.BoxStyle{
		PaddingRight:  "   ",
		UnfinishedRow: " ~",
	},
	Color:  table.ColorOptionsDefault,
	Format: table.FormatOptionsDefault,
	HTML:   table.DefaultHTMLOptions,
	Options: table.Options{
		DrawBorder:      false,
		SeparateColumns: false,
		SeparateFooter:  false,
		SeparateHeader:  false,
		SeparateRows:    false,
	},
	Size:  table.SizeOptionsDefault,
	Title: table.TitleOptionsDefault,
}

func FPrintComputeList(w io.Writer, lst []*compute.Instance, noHead bool) {
	tbl := table.NewWriter()
	tbl.SetOutputMirror(w)
	tbl.SetStyle(StyleK8s)
	if !noHead {
		tbl.AppendHeader(table.Row{"ID", "Name", "IP", "Status", "ZoneID", "SubnetID", "Platform"})
	}

	for _, item := range lst {
		tbl.AppendRow(table.Row{
			item.Id,
			item.Name,
			GetIPv4(item).PublicOrPrivate(false),
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
	tbl.AppendHeader(table.Row{"ComputeID", "Name", "IP", "Status", "ZoneID", "SubnetID", "Platform"})

	for _, item := range lst {
		tbl.AppendRow(table.Row{
			item.Id,
			item.Name,
			GetIPv4(item).PublicOrPrivate(false),
			item.Status.String(),
			item.ZoneId,
			item.NetworkInterfaces[0].SubnetId,
			item.PlatformId,
		})
	}

	tbl.Render()
}
