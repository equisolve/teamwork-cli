package cmd

import (
	"strings"
	"testing"
)

func TestTimerList_Empty(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/me/timers.json", `{"timers":[], "meta":{"page":{"count":0}}}`)

	out, _, code := runCLI(t, srv, "timer", "list")
	if code != 0 {
		t.Fatal("expected success")
	}
	if !strings.Contains(out, "0 timer(s)") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestTimerList_WithIncluded(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/me/timers.json", `{
		"timers": [
			{"id": 74120, "taskId": 28989564, "projectId": 445082,
			 "description": "Test work", "running": true, "billable": true,
			 "duration": 900}
		],
		"included": {
			"tasks":    {"28989564": {"id": 28989564, "name": "Email invoices"}},
			"projects": {"445082": {"id": 445082, "name": "Accounting"}}
		},
		"meta": {"page": {"count": 1}}
	}`)

	out, _, code := runCLI(t, srv, "timer", "list")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"74120", "Email invoices", "Accounting", "0h 15m", "running"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestTimerStart_WithTaskID(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("POST", "/projects/api/v3/me/timers.json", `{"timer":{"id":74121,"running":true}}`)

	out, _, code := runCLI(t, srv, "timer", "start", "--task", "28989564", "--description", "Smoke")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Timer 74121 started") {
		t.Errorf("unexpected output: %q", out)
	}
	if len(srv.calls) != 1 {
		t.Fatalf("calls = %+v", srv.calls)
	}
	if !strings.Contains(srv.calls[0].Body, `"taskId":28989564`) {
		t.Errorf("body = %q, missing taskId", srv.calls[0].Body)
	}
	if !strings.Contains(srv.calls[0].Body, `"description":"Smoke"`) {
		t.Errorf("body = %q, missing description", srv.calls[0].Body)
	}
}

func TestTimerStart_RequiresTaskOrProject(t *testing.T) {
	srv := newTestServer(t)
	_, errOut, code := runCLI(t, srv, "timer", "start")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(errOut, "--task or --project") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestTimerStop_PutsStopEndpoint(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("PUT", "/projects/api/v3/me/timers/74120/stop.json", `{"STATUS":"OK"}`)

	out, _, code := runCLI(t, srv, "timer", "stop", "74120")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "74120 stopped") {
		t.Errorf("unexpected: %q", out)
	}
}

func TestTimerDelete(t *testing.T) {
	srv := newTestServer(t)
	srv.handleStatus("DELETE", "/projects/api/v3/me/timers/74120.json", 204, ``)

	out, _, code := runCLI(t, srv, "timer", "delete", "74120")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "74120 deleted") {
		t.Errorf("unexpected: %q", out)
	}
}
