package cmd

import (
	"strings"
	"testing"
)

func TestCommentsList_OnTask(t *testing.T) {
	srv := newTestServer(t)
	// v1 comments response uses author-firstname + author-lastname (there is
	// no author-fullname field — the CLI used to read it and render blank).
	srv.handle("GET", "/tasks/42/comments.json", `{
		"comments": [
			{"id": "1", "author-firstname": "Ada", "author-lastname": "Lovelace",
			 "datetime": "2026-04-22T10:00:00Z", "body": "Looks good"}
		]
	}`)

	out, _, code := runCLI(t, srv, "comments", "list", "--on", "task", "--id", "42")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"Ada Lovelace", "Looks good", "2026-04-22"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %q", want, out)
		}
	}
}

func TestCommentsAdd_OnMilestone(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("POST", "/milestones/99/comments.json", `{"commentId":"101"}`)

	out, _, code := runCLI(t, srv, "comments", "add", "This is a test comment",
		"--on", "milestone", "--id", "99")
	if code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out, "Comment 101 posted on milestone 99") {
		t.Errorf("out = %q", out)
	}
	if !strings.Contains(srv.calls[0].Body, `"body":"This is a test comment"`) {
		t.Errorf("body = %q", srv.calls[0].Body)
	}
}

func TestCommentsAdd_UnknownKind(t *testing.T) {
	srv := newTestServer(t)
	_, errOut, code := runCLI(t, srv, "comments", "add", "hi", "--on", "widget", "--id", "1")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(errOut, "unknown resource kind") {
		t.Errorf("stderr = %q", errOut)
	}
}
