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
	// Verify the resolver translated "me" and passed responsiblePartyIds —
	// the v3 API silently ignores assignedToUserIds, so we use the supported
	// responsiblePartyIds parameter.
	sawAssignee := false
	for _, c := range srv.calls {
		if c.Path == "/projects/api/v3/tasks.json" && strings.Contains(c.Query, "responsiblePartyIds=86714") {
			sawAssignee = true
		}
	}
	if !sawAssignee {
		t.Errorf("expected tasks.json call with responsiblePartyIds=86714, got calls: %+v", srv.calls)
	}
}

func TestTasksList_DateRangeUsesISO(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/tasks.json", tasksListFixture)

	_, errOut, code := runCLI(t, srv, "tasks", "list", "--due-from", "2026-04-23", "--due-to", "2026-04-30")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	// v3 tasks.json wants ISO YYYY-MM-DD, not the compact YYYYMMDD v1 format.
	var q string
	for _, c := range srv.calls {
		if c.Path == "/projects/api/v3/tasks.json" {
			q = c.Query
		}
	}
	for _, want := range []string{"startDate=2026-04-23", "endDate=2026-04-30"} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q; got %q", want, q)
		}
	}
	for _, bad := range []string{"startDate=20260423", "endDate=20260430"} {
		if strings.Contains(q, bad) {
			t.Errorf("query still sends compact date %q: %s", bad, q)
		}
	}
}

func TestTasksList_CompletedFlag(t *testing.T) {
	// Fixture has one completed and one new task; --completed must filter to
	// only the completed one and map due-from/due-to to completedAfter/Before
	// (v3's startDate/endDate filter due date, not completion date).
	const fixture = `{
	  "tasks": [
	    {"id": 1, "name": "still open", "status": "new",
	     "dueDate": "2026-04-20T00:00:00Z"},
	    {"id": 2, "name": "all done", "status": "completed",
	     "dueDate": "2026-04-18T00:00:00Z"}
	  ],
	  "included": {"users": {}},
	  "meta": {"page": {"count": 2}}
	}`
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/tasks.json", fixture)

	out, errOut, code := runCLI(t, srv,
		"tasks", "list",
		"--completed",
		"--due-from", "2026-04-16",
		"--due-to", "2026-04-23",
	)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}

	var q string
	for _, c := range srv.calls {
		if c.Path == "/projects/api/v3/tasks.json" {
			q = c.Query
		}
	}
	for _, want := range []string{
		"includeCompletedTasks=true",
		"completedAfter=2026-04-16",
		"completedBefore=2026-04-23",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q; got %q", want, q)
		}
	}
	for _, bad := range []string{"startDate=", "endDate="} {
		if strings.Contains(q, bad) {
			t.Errorf("query should not send %q with --completed: %s", bad, q)
		}
	}
	if !strings.Contains(out, "all done") {
		t.Errorf("expected completed task in output:\n%s", out)
	}
	if strings.Contains(out, "still open") {
		t.Errorf("non-completed task should be filtered out client-side:\n%s", out)
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
