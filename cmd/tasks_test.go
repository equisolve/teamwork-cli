package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
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

func TestTasksComplete_BatchVariadic(t *testing.T) {
	// `tasks complete` should accept multiple IDs and PUT each one.
	srv := newTestServer(t)
	srv.handle("PUT", "/tasks/1/complete.json", `{"STATUS":"OK"}`)
	srv.handle("PUT", "/tasks/2/complete.json", `{"STATUS":"OK"}`)
	srv.handle("PUT", "/tasks/3/complete.json", `{"STATUS":"OK"}`)

	out, _, code := runCLI(t, srv, "tasks", "complete", "1", "2", "3")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "Completed 3 of 3") {
		t.Errorf("expected summary line, got %q", out)
	}
	// All three PUTs should have fired.
	gotPaths := map[string]bool{}
	for _, c := range srv.calls {
		gotPaths[c.Path] = true
	}
	for _, want := range []string{"/tasks/1/complete.json", "/tasks/2/complete.json", "/tasks/3/complete.json"} {
		if !gotPaths[want] {
			t.Errorf("missing call to %s; got %+v", want, srv.calls)
		}
	}
}

func TestTasksComplete_PredecessorErrorHints(t *testing.T) {
	// 422 with a predecessor message should surface the --with-predecessors hint
	// to stderr rather than retrying silently.
	srv := newTestServer(t)
	srv.handleStatus("PUT", "/tasks/42/complete.json", 422,
		`{"MESSAGE":"Cannot complete: task has incomplete predecessors."}`)

	_, errOut, code := runCLI(t, srv, "tasks", "complete", "42")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(errOut, "predecessor") {
		t.Errorf("expected predecessor message in stderr, got %q", errOut)
	}
	if !strings.Contains(errOut, "--with-predecessors") {
		t.Errorf("expected --with-predecessors hint in stderr, got %q", errOut)
	}
}

func TestTasksComplete_WithPredecessors(t *testing.T) {
	// First attempt to close 42 fails on predecessors; we then fetch task 42,
	// see predecessor 41, close 41, and retry 42.
	srv := newTestServer(t)
	// Two responses from /tasks/42/complete.json: first 422, then 200. The
	// test router doesn't support that natively; we approximate by ordering
	// — first matching route wins, so register 42 as 422 and rely on the
	// retry logic to re-call. Use a closure-based handler instead.
	attempts := 0
	srv.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		srv.mu.Lock()
		srv.calls = append(srv.calls, capturedCall{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body)})
		srv.mu.Unlock()
		switch {
		case r.Method == "PUT" && r.URL.Path == "/tasks/42/complete.json":
			attempts++
			if attempts == 1 {
				w.WriteHeader(422)
				fmt.Fprint(w, `{"MESSAGE":"Cannot complete: task has incomplete predecessors."}`)
				return
			}
			fmt.Fprint(w, `{"STATUS":"OK"}`)
		case r.Method == "GET" && r.URL.Path == "/projects/api/v3/tasks/42.json":
			fmt.Fprint(w, `{"task":{"id":42,"name":"Final","predecessors":[{"id":41,"completed":false}]}}`)
		case r.Method == "PUT" && r.URL.Path == "/tasks/41/complete.json":
			fmt.Fprint(w, `{"STATUS":"OK"}`)
		default:
			w.WriteHeader(404)
			fmt.Fprintf(w, `{"MESSAGE":"unmocked %s %s"}`, r.Method, r.URL.Path)
		}
	})

	out, errOut, code := runCLI(t, srv, "tasks", "complete", "42", "--with-predecessors")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "42 marked complete") {
		t.Errorf("expected 42 marked complete, got out=%q", out)
	}
	// Predecessor 41 should have been closed before the retry.
	saw41 := false
	for _, c := range srv.calls {
		if c.Method == "PUT" && c.Path == "/tasks/41/complete.json" {
			saw41 = true
		}
	}
	if !saw41 {
		t.Errorf("expected predecessor 41 to be closed, calls = %+v", srv.calls)
	}
	if attempts < 2 {
		t.Errorf("expected 42 to be retried, attempts=%d", attempts)
	}
}

func TestTasksComplete_FromStdin(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("PUT", "/tasks/100/complete.json", `{"STATUS":"OK"}`)
	srv.handle("PUT", "/tasks/200/complete.json", `{"STATUS":"OK"}`)

	old := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = old }()
	go func() {
		fmt.Fprintln(w, "100")
		fmt.Fprintln(w, "# a comment")
		fmt.Fprintln(w, "200")
		w.Close()
	}()

	out, errOut, code := runCLI(t, srv, "tasks", "complete", "--from-stdin")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	for _, want := range []string{"100 marked complete", "200 marked complete", "Completed 2 of 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}
