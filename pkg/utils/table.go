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

package utils

import (
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
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

type Table struct {
	Header table.Row
	Rows   []table.Row
}

func NewTable() *Table {
	return &Table{}
}

func (t *Table) SetHeader(v ...any) {
	t.Header = v
}

func (t *Table) AddRow(v ...any) {
	t.Rows = append(t.Rows, v)
}

func (t *Table) Render(w io.Writer) {
	tbl := table.NewWriter()
	tbl.SetOutputMirror(w)
	tbl.SetStyle(StyleK8s)

	if t.Header != nil {
		tbl.AppendHeader(t.Header)
	}

	tbl.AppendRows(t.Rows)
	tbl.Render()
}
