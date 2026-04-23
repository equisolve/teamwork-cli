package cmd

import (
	"strings"
	"testing"
)

const projectsListFixture = `{
  "projects": [
    {"id": 795529, "name": "Acme Website Redesign", "status": "active",
     "company": {"id": 167137}, "endAt": "2027-06-21T00:00:00Z",
     "updatedAt": "2026-04-22T13:10:44Z"},
    {"id": 788239, "name": "Internal Tools", "status": "active",
     "company": {"id": 43901}, "endAt": "", "updatedAt": "2026-04-13T00:00:00Z"}
  ],
  "included": {
    "companies": {
      "167137": {"id": 167137, "name": "Acme Corp"},
      "43901":  {"id": 43901, "name": "Example Co"}
    }
  },
  "meta": {"page": {"pageOffset": 0, "pageSize": 50, "count": 2, "hasMore": false}}
}`

func TestProjectsList_Table(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/projects.json", projectsListFixture)

	out, errOut, code := runCLI(t, srv, "projects", "list")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Acme Website Redesign") {
		t.Errorf("missing project name in output:\n%s", out)
	}
	if !strings.Contains(out, "Example Co") {
		t.Errorf("missing sideloaded company name:\n%s", out)
	}
	if !strings.Contains(out, "2 of 2 project(s)") {
		t.Errorf("missing pagination footer:\n%s", out)
	}
}

func TestProjectsList_JSON(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/projects.json", projectsListFixture)

	out, _, code := runCLI(t, srv, "projects", "list", "--json")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if got := jsonField(t, out, "projects.0.name"); got != "Acme Website Redesign" {
		t.Errorf("json projects[0].name = %q", got)
	}
}

func TestProjectsList_APIError(t *testing.T) {
	srv := newTestServer(t)
	srv.handleStatus("GET", "/projects/api/v3/projects.json", 401, `{"MESSAGE":"invalid token"}`)

	_, errOut, code := runCLI(t, srv, "projects", "list")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(errOut, "Invalid API token") {
		t.Errorf("stderr missing friendly 401 message: %q", errOut)
	}
}

func TestProjectsShow_Table(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/projects/795529.json", `{
		"project": {
			"id": 795529, "name": "Acme Website Redesign",
			"status": "active", "subStatus": "current",
			"description": "Test",
			"startAt": "2025-08-27T00:00:00Z", "endAt": "2027-06-21T00:00:00Z",
			"company": {"id": 167137}
		},
		"included": {"companies": {"167137": {"id": 167137, "name": "Acme Corp"}}}
	}`)

	out, _, code := runCLI(t, srv, "projects", "show", "795529")
	if code != 0 {
		t.Fatal("expected success")
	}
	for _, want := range []string{"ID:", "795529", "Acme", "active", "2025-08-27"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}
