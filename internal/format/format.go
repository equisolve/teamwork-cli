package format

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

type OutputMode string

const (
	Table OutputMode = "table"
	JSON  OutputMode = "json"
	CSV   OutputMode = "csv"
)

func ParseMode(s string) OutputMode {
	switch strings.ToLower(s) {
	case "json":
		return JSON
	case "csv":
		return CSV
	default:
		return Table
	}
}

func PrintJSON(data json.RawMessage) {
	var out interface{}
	if json.Unmarshal(data, &out) == nil {
		formatted, err := json.MarshalIndent(out, "", "  ")
		if err == nil {
			fmt.Println(string(formatted))
			return
		}
	}
	fmt.Println(string(data))
}

func PrintTable(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No results.")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)

	headerRow := make(table.Row, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}
	t.AppendHeader(headerRow)

	for _, row := range rows {
		r := make(table.Row, len(row))
		for i, v := range row {
			r[i] = v
		}
		t.AppendRow(r)
	}

	t.SetStyle(table.Style{
		Name: "clean",
		Box: table.BoxStyle{
			PaddingLeft:  "",
			PaddingRight: "  ",
		},
		Format: table.FormatOptions{
			Header: text.FormatUpper,
		},
		Options: table.Options{
			DrawBorder:      false,
			SeparateColumns: false,
			SeparateHeader:  false,
			SeparateRows:    false,
		},
	})

	t.Render()
}

func PrintCSV(headers []string, rows [][]string) {
	w := csv.NewWriter(os.Stdout)
	_ = w.Write(headers)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
}

func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// DurationMinutes formats a total-minute count as "Xh MMm". Negative durations
// render with a leading minus. Zero renders as "0h 00m".
func DurationMinutes(totalMin int) string {
	sign := ""
	if totalMin < 0 {
		sign = "-"
		totalMin = -totalMin
	}
	return fmt.Sprintf("%s%dh %02dm", sign, totalMin/60, totalMin%60)
}
