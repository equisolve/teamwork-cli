package cmd

import (
	"strings"
	"testing"
)

func TestTasksCreate_RequiresTasklistAndName(t *testing.T) {
	srv := newTestServer(t)
	_, errOut, code := runCLI(t, srv, "tasks", "create")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(errOut, "--tasklist and --name are required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestTasksCreate_PostsToTasklistEndpoint(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("POST", "/tasklists/1702954/tasks.json", `{"id":"42","taskId":"42"}`)

	out, _, code := runCLI(t, srv, "tasks", "create",
		"--tasklist", "1702954", "--name", "CLI smoke test",
		"--priority", "high", "--due", "2026-05-01")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Task created: 42") {
		t.Errorf("output = %q", out)
	}
	body := srv.calls[0].Body
	for _, want := range []string{`"content":"CLI smoke test"`, `"priority":"high"`, `"due-date":"20260501"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q, missing %q", body, want)
		}
	}
}

func TestTasksUpdate_RequiresAtLeastOneField(t *testing.T) {
	srv := newTestServer(t)
	_, errOut, code := runCLI(t, srv, "tasks", "update", "42")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(errOut, "no updates specified") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestTasksUpdate_PutsTaskBody(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("PUT", "/tasks/42.json", `{"STATUS":"OK"}`)

	_, _, code := runCLI(t, srv, "tasks", "update", "42", "--priority", "low", "--name", "New name")
	if code != 0 {
		t.Fatal(code)
	}
	body := srv.calls[0].Body
	for _, want := range []string{`"content":"New name"`, `"priority":"low"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q, missing %q", body, want)
		}
	}
}

func TestTasksDelete_WithYesFlag(t *testing.T) {
	srv := newTestServer(t)
	srv.handleStatus("DELETE", "/tasks/42.json", 200, `{"STATUS":"OK"}`)

	out, _, code := runCLI(t, srv, "tasks", "delete", "42", "--yes")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "42 deleted") {
		t.Errorf("out = %q", out)
	}
}

func TestTasksUncomplete(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("PUT", "/tasks/42/uncomplete.json", `{"STATUS":"OK"}`)
	out, _, code := runCLI(t, srv, "tasks", "uncomplete", "42")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "42 reopened") {
		t.Errorf("out = %q", out)
	}
}

func TestTasksSweep_DryRunBuckets(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/tasks.json", `{
		"tasks": [
			{"id": 1, "name": "Done: ship homepage", "status": "new"},
			{"id": 2, "name": "N/A: legacy CMS export", "status": "new"},
			{"id": 3, "name": "QA: navigation review", "status": "new"},
			{"id": 4, "name": "Wire up product feed", "status": "new"},
			{"id": 5, "name": "Already shipped", "status": "completed"}
		],
		"meta": {"page": {"count": 5}}
	}`)

	out, errOut, code := runCLI(t, srv, "tasks", "sweep", "--tasklist", "999")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	for _, want := range []string{
		"Done (1)", "N/A (1)", "QA (1)", "Blocked (0)",
		"Unbucketed (1)", "Wire up product feed",
		"Dry run only", // status reminder
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	// Completed tasks shouldn't appear at all.
	if strings.Contains(out, "Already shipped") {
		t.Errorf("completed task leaked into sweep output:\n%s", out)
	}
}

func TestTasksSweep_CloseBucket(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/tasks.json", `{
		"tasks": [
			{"id": 1, "name": "Done: ship homepage", "status": "new"},
			{"id": 2, "name": "QA: footer", "status": "new"}
		],
		"meta": {"page": {"count": 2}}
	}`)
	srv.handle("PUT", "/tasks/1/complete.json", `{"STATUS":"OK"}`)

	out, errOut, code := runCLI(t, srv,
		"tasks", "sweep", "--tasklist", "999", "--close", "done", "--yes")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Closed 1 of 1") {
		t.Errorf("expected close summary, got %q", out)
	}
	// QA task should have been left alone — verify the only PUT was for task 1.
	saw1, sawOther := false, false
	for _, c := range srv.calls {
		if c.Method != "PUT" {
			continue
		}
		switch c.Path {
		case "/tasks/1/complete.json":
			saw1 = true
		default:
			sawOther = true
		}
	}
	if !saw1 || sawOther {
		t.Errorf("expected only PUT /tasks/1/complete.json, calls=%+v", srv.calls)
	}
}

func TestTasksSubtasks_Add(t *testing.T) {
	// v1 /tasks/<id>/quickadd.json only accepts a single "content" string
	// (it was never the right endpoint for multiple subtasks — the wrapped
	// `{"todo-item":{"content":...}}` shape it used returned a 400 anyway).
	// We now split on newline / ~|~ and POST one v3 subtask per line.
	srv := newTestServer(t)
	srv.handle("POST", "/projects/api/v3/tasks/42/subtasks.json", `{"task":{"id":1}}`)

	out, _, code := runCLI(t, srv, "tasks", "subtasks", "42", "--add", "one\ntwo\nthree")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Added 3 subtask(s)") {
		t.Errorf("out = %q", out)
	}
	if len(srv.calls) != 3 {
		t.Fatalf("expected 3 POSTs, got %d: %+v", len(srv.calls), srv.calls)
	}
	for i, want := range []string{"one", "two", "three"} {
		if !strings.Contains(srv.calls[i].Body, `"name":"`+want+`"`) {
			t.Errorf("call %d body missing name=%q: %q", i, want, srv.calls[i].Body)
		}
	}
}
