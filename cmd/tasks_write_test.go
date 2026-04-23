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
