package cmd

import (
	"strings"
	"testing"
)

func TestTimeUpdate_PutsBody(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("PUT", "/time_entries/1234.json", `{"STATUS":"OK"}`)

	_, _, code := runCLI(t, srv, "time", "update", "1234",
		"--hours", "2", "--minutes", "15", "--description", "Updated",
		"--date", "2026-04-22", "--billable", "no")
	if code != 0 {
		t.Fatal(code)
	}
	body := srv.calls[0].Body
	for _, want := range []string{`"hours":"2"`, `"minutes":"15"`, `"description":"Updated"`, `"date":"20260422"`, `"isbillable":"0"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q, missing %q", body, want)
		}
	}
}

func TestTimeUpdate_RequiresAtLeastOne(t *testing.T) {
	srv := newTestServer(t)
	_, errOut, code := runCLI(t, srv, "time", "update", "1234")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(errOut, "no updates specified") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestTimeDelete_WithYes(t *testing.T) {
	srv := newTestServer(t)
	srv.handleStatus("DELETE", "/time_entries/1234.json", 200, `{"STATUS":"OK"}`)
	out, _, code := runCLI(t, srv, "time", "delete", "1234", "--yes")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "1234 deleted") {
		t.Errorf("out = %q", out)
	}
}
