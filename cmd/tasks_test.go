package cmd

import (
	"strings"
	"testing"
)

const tasksListFixture = `{
  "tasks": [
    {"id": 28989564, "name": "Email Press Release Invoices Imported",
     "status": "new", "priority": "high",
     "dueDate": "2026-04-22T00:00:00Z",
     "assignees": [{"id": 120554, "type": "users"}],
     "tasklistId": 1702954},
    {"id": 28976675, "name": "CS Additional work reports to Mariah",
     "status": "new", "priority": "",
     "dueDate": "2026-05-08T00:00:00Z",
     "assignees": [{"id": 120507, "type": "users"}, {"id": 120515, "type": "users"}],
     "tasklistId": 1702954}
  ],
  "included": {
    "users": {
      "120554": {"id": 120554, "firstName": "Teresa", "lastName": "Schelling"},
      "120507": {"id": 120507, "firstName": "Dana", "lastName": "Clore"},
      "120515": {"id": 120515, "firstName": "Agustina", "lastName": "Prigoshin"}
    }
  },
  "meta": {"page": {"count": 2, "hasMore": false}}
}`

func TestTasksList_Table(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/tasks.json", tasksListFixture)

	out, errOut, code := runCLI(t, srv, "tasks", "list")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	for _, want := range []string{"Teresa Schelling", "Dana Clore (+1)", "high", "2026-04-22"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestTasksList_FilterByAssigneeMe(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/me.json", `{"person":{"id":"86714"}}`)
	srv.handle("GET", "/projects/api/v3/tasks.json", tasksListFixture)

	_, errOut, code := runCLI(t, srv, "tasks", "list", "--assignee", "me")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	// Verify the resolver translated "me" and passed assignedToUserIds.
	sawAssignee := false
	for _, c := range srv.calls {
		if c.Path == "/projects/api/v3/tasks.json" && strings.Contains(c.Query, "assignedToUserIds=86714") {
			sawAssignee = true
		}
	}
	if !sawAssignee {
		t.Errorf("expected tasks.json call with assignedToUserIds=86714, got calls: %+v", srv.calls)
	}
}

func TestTasksShow_Table(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/tasks/42.json", `{
		"task": {
			"id": 42, "name": "Do a thing", "status": "new", "priority": "medium",
			"dueDate": "2026-05-01T00:00:00Z",
			"assignees": [{"id": 86714, "type": "users"}]
		},
		"included": {"users": {"86714": {"firstName": "Ada", "lastName": "Lovelace"}}}
	}`)

	out, _, code := runCLI(t, srv, "tasks", "show", "42")
	if code != 0 {
		t.Fatal("expected success")
	}
	for _, want := range []string{"Do a thing", "Ada Lovelace", "2026-05-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestTasksComplete_PutsCompleteEndpoint(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("PUT", "/tasks/42/complete.json", `{"STATUS":"OK"}`)

	out, _, code := runCLI(t, srv, "tasks", "complete", "42")
	if code != 0 {
		t.Fatal("expected success")
	}
	if !strings.Contains(out, "42 marked complete") {
		t.Errorf("unexpected output: %q", out)
	}
	// Confirm the PUT landed on the right path.
	if len(srv.calls) != 1 || srv.calls[0].Method != "PUT" || srv.calls[0].Path != "/tasks/42/complete.json" {
		t.Errorf("calls = %+v", srv.calls)
	}
}
