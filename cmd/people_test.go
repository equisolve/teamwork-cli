package cmd

import (
	"strings"
	"testing"
)

func TestPeopleList_Table(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/people.json", `{
		"people": [
			{"id": 12345, "firstName": "Ada", "lastName": "Lovelace",
			 "email": "ada@example.com", "companyId": 43901,
			 "title": "VP Engineering", "lastLogin": "2026-04-22T21:00:00Z"}
		],
		"included": {"companies": {"43901": {"id": 43901, "name": "Example Co"}}},
		"meta": {"page": {"count": 1}}
	}`)

	out, _, code := runCLI(t, srv, "people", "list")
	if code != 0 {
		t.Fatal("expected success")
	}
	for _, want := range []string{"Ada Lovelace", "ada@example.com", "Example Co", "VP Engineering"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestPeopleShow_ResolvesMe(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/me.json", `{"person":{"id":"12345"}}`)
	srv.handle("GET", "/projects/api/v3/people/12345.json", `{
		"person": {"id": 12345, "firstName": "Ada", "lastName": "Lovelace",
		           "email": "ada@example.com"},
		"included": {}
	}`)

	out, _, code := runCLI(t, srv, "people", "show", "me")
	if code != 0 {
		t.Fatal("expected success")
	}
	if !strings.Contains(out, "Ada") || !strings.Contains(out, "ada@example.com") {
		t.Errorf("unexpected output:\n%s", out)
	}
}
