package cmd

import (
	"strings"
	"testing"
)

func TestNotebooksShow_ContentFromV3(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/notebooks/55.json", `{
		"notebook": {"id": 55, "name": "Project Brief",
		             "contents": "<h2>Database</h2><p>db_acme</p><h2>URL</h2><p>acme.test</p>"}
	}`)
	out, _, code := runCLI(t, srv, "notebooks", "show", "55", "--content")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "<h2>Database</h2>") {
		t.Errorf("expected raw HTML body, got %q", out)
	}
	// Header lines should NOT print when --content is set.
	if strings.Contains(out, "ID:") || strings.Contains(out, "Name:") {
		t.Errorf("--content should suppress header rows; got %q", out)
	}
}

func TestNotebooksShow_PlainStripsTags(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/notebooks/55.json", `{
		"notebook": {"id": 55, "name": "Brief",
		             "contents": "<h2>Database</h2><p>db_acme</p>"}
	}`)
	out, _, code := runCLI(t, srv, "notebooks", "show", "55", "--content", "--plain")
	if code != 0 {
		t.Fatal(code)
	}
	if strings.Contains(out, "<") || strings.Contains(out, ">") {
		t.Errorf("--plain should strip tags, got %q", out)
	}
	if !strings.Contains(out, "Database") || !strings.Contains(out, "db_acme") {
		t.Errorf("expected stripped text, got %q", out)
	}
}

func TestNotebooksShow_SectionExtract(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/notebooks/55.json", `{
		"notebook": {"contents":
			"<h2>Database</h2><p>db_acme</p><h2>URL</h2><p>acme.test</p><h2>Launch</h2><p>2026-06-01</p>"
		}
	}`)
	out, _, code := runCLI(t, srv, "notebooks", "show", "55", "--content", "--plain", "--section", "URL")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "acme.test") {
		t.Errorf("expected URL-section content, got %q", out)
	}
	if strings.Contains(out, "db_acme") || strings.Contains(out, "2026-06-01") {
		t.Errorf("section extract bled into other sections: %q", out)
	}
}

func TestNotebooksShow_V1Fallback(t *testing.T) {
	// v3 returns notebook metadata but no body. We should hit /notebooks/<id>.json
	// (v1) with includeContent=true to recover the HTML.
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/notebooks/55.json", `{
		"notebook": {"id": 55, "name": "Old Brief", "type": "html"}
	}`)
	srv.handle("GET", "/notebooks/55.json", `{
		"notebook": {"content": "<p>recovered from v1</p>"}
	}`)
	out, _, code := runCLI(t, srv, "notebooks", "show", "55", "--content", "--plain")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "recovered from v1") {
		t.Errorf("expected v1-fallback body, got %q", out)
	}
	// And the v1 fallback URL should have been called with includeContent.
	saw := false
	for _, c := range srv.calls {
		if c.Path == "/notebooks/55.json" && strings.Contains(c.Query, "includeContent=true") {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected v1 fallback request with includeContent=true, calls=%+v", srv.calls)
	}
}
