package format

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := map[string]OutputMode{
		"table":    Table,
		"TABLE":    Table,
		"json":     JSON,
		"JSON":     JSON,
		"csv":      CSV,
		"CSV":      CSV,
		"unknown":  Table,
		"":         Table,
	}
	for in, want := range cases {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		s      string
		max    int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "hi"},
		{"hello", 3, "hel"},
		{"", 5, ""},
	}
	for _, tc := range cases {
		if got := Truncate(tc.s, tc.max); got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.s, tc.max, got, tc.want)
		}
	}
}

func TestDurationMinutes(t *testing.T) {
	cases := []struct {
		min  int
		want string
	}{
		{0, "0h 00m"},
		{1, "0h 01m"},
		{59, "0h 59m"},
		{60, "1h 00m"},
		{90, "1h 30m"},
		{125, "2h 05m"},
		{-90, "-1h 30m"},
	}
	for _, tc := range cases {
		if got := DurationMinutes(tc.min); got != tc.want {
			t.Errorf("DurationMinutes(%d) = %q, want %q", tc.min, got, tc.want)
		}
	}
}

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	PrintTable(&buf, []string{"A", "B"}, nil)
	if !strings.Contains(buf.String(), "No results.") {
		t.Errorf("empty table = %q, want 'No results.'", buf.String())
	}
}

func TestPrintTable_RendersRows(t *testing.T) {
	var buf bytes.Buffer
	PrintTable(&buf, []string{"ID", "NAME"}, [][]string{
		{"1", "Alice"},
		{"2", "Bob"},
	})
	out := buf.String()
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Bob") {
		t.Errorf("table missing data: %q", out)
	}
	if !strings.Contains(out, "ID") || !strings.Contains(out, "NAME") {
		t.Errorf("table missing headers: %q", out)
	}
}

func TestPrintJSON_Indented(t *testing.T) {
	// capture stdout
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	PrintJSON([]byte(`{"a":1,"b":[2,3]}`))
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "\n") || !strings.Contains(out, "  ") {
		t.Errorf("expected indented output, got %q", out)
	}
}

func TestPrintCSV(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	PrintCSV([]string{"id", "name"}, [][]string{
		{"1", "Alice,Wonder"},
		{"2", "Bob"},
	})
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "id,name") {
		t.Errorf("csv missing header: %q", out)
	}
	// CSV should quote values containing commas.
	if !strings.Contains(out, `"Alice,Wonder"`) {
		t.Errorf("csv did not escape comma: %q", out)
	}
}
