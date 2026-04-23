package cmd

import (
	"strings"
	"testing"
)

func TestTasklistsList_ByProjectName(t *testing.T) {
	srv := newTestServer(t)
	// Resolver searches v1 /projects.json to turn name into ID.
	srv.handle("GET", "/projects.json", `{"projects":[{"id":"445082","name":"Accounting"}]}`)
	srv.handle("GET", "/projects/445082/tasklists.json", `{
		"tasklists": [
			{"id": "100", "name": "Admin", "complete": false, "uncompleted-count": "4"},
			{"id": "101", "name": "Reports", "complete": true,  "uncompleted-count": "0"}
		]
	}`)

	out, _, code := runCLI(t, srv, "tasklists", "list", "--project", "Accounting")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"Admin", "Reports", "100", "101"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestTasklistsShow(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/tasklists/100.json", `{
		"todo-list": {
			"id": "100", "name": "Admin",
			"description": "Weekly tasks",
			"complete": "false", "uncompleted-count": "4"
		}
	}`)
	out, _, code := runCLI(t, srv, "tasklists", "show", "100")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"Admin", "Weekly tasks"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}
