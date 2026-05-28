package cmd

import (
	"strings"
	"testing"
)

func TestAPI_Get(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects.json", `{"projects":[{"id":"1","name":"Alpha"}]}`)

	out, _, code := runCLI(t, srv, "api", "/projects.json")
	if code != 0 {
		t.Fatalf("expected success, got %d", code)
	}
	if !strings.Contains(out, "Alpha") {
		t.Errorf("missing project name:\n%s", out)
	}
}

func TestAPI_AddsLeadingSlashAndQuery(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects.json", `{"projects":[]}`)

	_, _, code := runCLI(t, srv, "api", "projects.json", "-q", "status=active", "-q", "page=2")
	if code != 0 {
		t.Fatalf("expected success, got %d", code)
	}
	call := srv.calls[len(srv.calls)-1]
	if call.Path != "/projects.json" {
		t.Errorf("expected leading slash added, got path %q", call.Path)
	}
	if !strings.Contains(call.Query, "status=active") || !strings.Contains(call.Query, "page=2") {
		t.Errorf("query params not forwarded: %q", call.Query)
	}
}

func TestAPI_PostBody(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("POST", "/tasks/123/todo-items.json", `{"id":"99"}`)

	_, _, code := runCLI(t, srv, "api", "-X", "POST", "-d", `{"todo-item":{"content":"Hi"}}`, "/tasks/123/todo-items.json")
	if code != 0 {
		t.Fatalf("expected success, got %d", code)
	}
	call := srv.calls[len(srv.calls)-1]
	if call.Method != "POST" {
		t.Errorf("expected POST, got %s", call.Method)
	}
	if !strings.Contains(call.Body, `"content":"Hi"`) {
		t.Errorf("body not forwarded: %q", call.Body)
	}
}

func TestAPI_BadQueryParam(t *testing.T) {
	srv := newTestServer(t)
	_, stderr, code := runCLI(t, srv, "api", "/me.json", "-q", "noequalshere")
	if code == 0 {
		t.Fatal("expected failure on malformed query param")
	}
	if !strings.Contains(stderr, "key=value") {
		t.Errorf("expected helpful error, got: %s", stderr)
	}
}
