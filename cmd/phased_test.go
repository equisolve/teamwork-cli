package cmd

// Aggregate happy-path tests for all Phase D read-only resources. One test per
// command verifies the endpoint is hit and output contains key fields.

import (
	"strings"
	"testing"
)

func TestMilestonesList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/milestones.json", `{
		"milestones": [{"id": 1, "name": "Go-Live", "projectId": 10, "deadline": "2026-05-01T00:00:00Z", "status": "upcoming"}],
		"included": {"projects": {"10": {"id": 10, "name": "Acme"}}},
		"meta": {"page": {"count": 1}}
	}`)
	out, _, code := runCLI(t, srv, "milestones", "list")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"Go-Live", "Acme", "upcoming", "2026-05-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestMessagesList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/messages.json", `{
		"messages": [{"id": 5, "title": "Launch plan", "status": "active",
			"projectId": 10, "author": {"id": 100}, "createdAt": "2026-04-22T10:00:00Z"}],
		"included": {
			"projects": {"10": {"id": 10, "name": "Acme"}},
			"users":    {"100": {"id": 100, "firstName": "Ada", "lastName": "Lovelace"}}
		},
		"meta": {"page": {"count": 1}}
	}`)
	out, _, code := runCLI(t, srv, "messages", "list")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"Launch plan", "Ada Lovelace", "Acme"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestFilesList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/files.json", `{
		"files": [{"id": 7, "displayName": "spec.pdf", "originalName": "spec.pdf",
			"latestFileVersionNo": 3, "projectId": 10, "description": "Design spec"}],
		"included": {"projects": {"10": {"id": 10, "name": "Acme"}}},
		"meta": {"page": {"count": 1}}
	}`)
	out, _, code := runCLI(t, srv, "files", "list")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"spec.pdf", "Acme", "Design spec"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestFilesList_ByProjectName(t *testing.T) {
	// v3 /files.json ignores `projectIds`, so when the caller scopes by
	// project we must hit the v1 /projects/<id>/files.json endpoint.
	// Fixture uses the real v1 nested `project.files[]` shape.
	srv := newTestServer(t)
	srv.handle("GET", "/projects.json", `{"projects":[{"id":"10","name":"Acme"}]}`)
	srv.handle("GET", "/projects/10/files.json", `{
		"project": {
			"id": "10", "name": "Acme",
			"files": [
				{"id": "7", "name": "spec.pdf", "originalName": "spec.pdf",
				 "version": "3", "project-id": "10", "description": "Design spec"}
			]
		}
	}`)
	out, _, code := runCLI(t, srv, "files", "list", "--project", "Acme")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"spec.pdf", "Acme", "Design spec"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestTimeList_PersonColumn(t *testing.T) {
	// v1 /time_entries.json returns `person-first-name` + `person-last-name`
	// — not a combined `person-full-name`, which is what the parser used to
	// read and which left the PERSON column blank in production.
	srv := newTestServer(t)
	srv.handle("GET", "/time_entries.json", `{
		"time-entries": [
			{"id": "1", "date": "2026-04-22T00:00:00Z",
			 "person-first-name": "Ada", "person-last-name": "Lovelace",
			 "project-name": "Acme", "todo-item-name": "Do X",
			 "hours": "1", "minutes": "30", "isbillable": "1"}
		]
	}`)
	out, _, code := runCLI(t, srv, "time", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Ada Lovelace") {
		t.Errorf("PERSON column should show 'Ada Lovelace':\n%s", out)
	}
}

func TestNotebooksList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/notebooks.json", `{
		"notebooks": [{"id": 11, "name": "Runbook", "projectId": 10, "type": "HTML"}],
		"included": {"projects": {"10": {"id": 10, "name": "Acme"}}},
		"meta": {"page": {"count": 1}}
	}`)
	out, _, code := runCLI(t, srv, "notebooks", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Runbook") || !strings.Contains(out, "Acme") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestLinksList_ByProjectName(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects.json", `{"projects":[{"id":"10","name":"Acme"}]}`)
	srv.handle("GET", "/projects/10/links.json", `{
		"links": [{"id": "1", "name": "Jira board", "url": "https://example.com", "category-name": "Docs"}]
	}`)
	out, _, code := runCLI(t, srv, "links", "list", "--project", "Acme")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Jira board") || !strings.Contains(out, "Docs") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestSearch_Tasks(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/search.json", `{
		"searchResult": {
			"tasks": [{"id": 42, "name": "Do X", "projectName": "Acme"}]
		}
	}`)
	out, _, code := runCLI(t, srv, "search", "Do X")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Do X") || !strings.Contains(out, "Acme") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestActivity_Global(t *testing.T) {
	// Fixture mirrors a live /latestActivity.json row:
	//   `activitytype` (verb), `type` (object), `project-name`, `datetime`,
	//   `fromusername` (actor), `forusername` (target — empty for most
	//   actions like `completed`).
	// The USER column must surface the actor; reading `forusername` would
	// leave it blank here, just like it does in production.
	srv := newTestServer(t)
	srv.handle("GET", "/latestActivity.json", `{
		"activity": [
			{"id": "1", "activitytype": "completed", "type": "task",
			 "datetime": "2026-04-22T10:00:00Z", "project-name": "Acme",
			 "fromusername": "Ada Lovelace", "forusername": "",
			 "description": "Ship it"}
		]
	}`)
	out, _, code := runCLI(t, srv, "activity")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"Acme", "Ada Lovelace", "completed", "task", "Ship it"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestTagsList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/tags.json", `{
		"tags": [{"id": "1", "name": "urgent", "color": "#ff0000", "projectId": "0", "dateCreated": "2026-01-01T00:00:00Z"}]
	}`)
	out, _, code := runCLI(t, srv, "tags", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "urgent") || !strings.Contains(out, "#ff0000") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestCategoriesList_Project(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projectCategories.json", `{
		"categories": [{"id": "1", "name": "AaaS", "color": "#abc", "count": "15"}]
	}`)
	out, _, code := runCLI(t, srv, "categories", "list", "--kind", "project")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "AaaS") || !strings.Contains(out, "15") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestInvoicesList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/invoices.json", `{
		"invoices": [{"id": 1, "number": "INV-001", "status": "open", "projectId": 10,
		              "total": 1500.50, "currencyCode": "USD", "displayDate": "2026-04-01"}],
		"included": {"projects": {"10": {"id": 10, "name": "Acme"}}},
		"meta": {"page": {"count": 1}}
	}`)
	out, _, code := runCLI(t, srv, "invoices", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "INV-001") || !strings.Contains(out, "USD 1500.50") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestExpensesList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/expenses.json", `{
		"expenses": [{"id": 1, "name": "AWS", "cost": 42.00, "currencyCode": "USD",
		              "projectId": 10, "date": "2026-04-01T00:00:00Z"}],
		"included": {"projects": {"10": {"id": 10, "name": "Acme"}}},
		"meta": {"page": {"count": 1}}
	}`)
	out, _, code := runCLI(t, srv, "expenses", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "AWS") || !strings.Contains(out, "USD 42.00") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestRisksList_Empty(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/risks.json", `{"risks":[]}`)
	out, _, code := runCLI(t, srv, "risks", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "No results") && !strings.Contains(out, "0 risk(s)") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestTemplatesList(t *testing.T) {
	srv := newTestServer(t)
	// v3 returns templates under the "projects" key (templates are projects with
	// isTemplate=true), captured from a live response.
	srv.handle("GET", "/projects/api/v3/projects/templates.json", `{
		"projects": [{"id": 1, "name": "Standard IR", "description": "Investor relations template"}],
		"meta": {"page": {"count": 1}}
	}`)
	out, _, code := runCLI(t, srv, "templates", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Standard IR") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestPortfolioBoardsList(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/portfolio/boards.json", `{
		"boards": [{"id": "1", "name": "All Projects", "description": ""}]
	}`)
	out, _, code := runCLI(t, srv, "portfolio", "boards", "list")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "All Projects") {
		t.Errorf("unexpected:\n%s", out)
	}
}
