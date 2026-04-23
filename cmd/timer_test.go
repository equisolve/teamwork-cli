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
	// Task lookup is called to derive projectId — the v3 task resource has
	// no top-level projectId; the project arrives via included sideload.
	srv.handle("GET", "/projects/api/v3/tasks/28989564.json", `{
		"task": {"id":28989564,"tasklistId":1702954},
		"included": {"projects": {"445082": {"id": 445082, "name": "Accounting"}}}
	}`)
	srv.handle("POST", "/projects/api/v3/me/timers.json", `{"timer":{"id":74121,"running":true}}`)

	out, _, code := runCLI(t, srv, "timer", "start", "--task", "28989564", "--description", "Smoke")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Timer 74121 started") {
		t.Errorf("unexpected output: %q", out)
	}
	// Locate the POST /timers.json call (task lookup fires first).
	var postBody string
	for _, c := range srv.calls {
		if c.Method == "POST" && c.Path == "/projects/api/v3/me/timers.json" {
			postBody = c.Body
		}
	}
	if postBody == "" {
		t.Fatalf("no POST to timers.json; calls = %+v", srv.calls)
	}
	if !strings.Contains(postBody, `"description":"Smoke"`) {
		t.Errorf("body = %q, missing description", postBody)
	}
	// v3 timer endpoint accepts isBillable, not billable (the bare name 400s).
	if !strings.Contains(postBody, `"isBillable":`) {
		t.Errorf("body = %q, missing isBillable", postBody)
	}
	if strings.Contains(postBody, `"billable":`) {
		t.Errorf("body = %q, should not send raw billable key", postBody)
	}
	if !strings.Contains(postBody, `"taskId":28989564`) {
		t.Errorf("body = %q, missing taskId", postBody)
	}
	// projectId must be derived from the task so /complete.json works later.
	if !strings.Contains(postBody, `"projectId":445082`) {
		t.Errorf("body = %q, expected projectId derived from task", postBody)
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

func TestTimerStop_PutsCompleteEndpoint(t *testing.T) {
	// v3 "stop and log" is served by /complete.json. The old /stop.json
	// path returned 404.
	srv := newTestServer(t)
	srv.handle("PUT", "/projects/api/v3/me/timers/74120/complete.json", `{"STATUS":"OK"}`)

	out, _, code := runCLI(t, srv, "timer", "stop", "74120")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "74120 stopped") {
		t.Errorf("unexpected: %q", out)
	}
	if len(srv.calls) != 1 || srv.calls[0].Path != "/projects/api/v3/me/timers/74120/complete.json" {
		t.Errorf("expected PUT to /complete.json, got calls = %+v", srv.calls)
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
