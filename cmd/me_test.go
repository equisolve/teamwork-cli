package cmd

import (
	"strings"
	"testing"
)

func TestMe_Table(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/me.json", `{
		"person": {
			"id": "12345",
			"full-name": "Ada Lovelace",
			"email-address": "ada@example.com",
			"company-name": "Example Co",
			"administrator": true,
			"user-timezone": "America/New_York",
			"last-login": "2026-04-22T21:58:31Z"
		}
	}`)

	out, _, code := runCLI(t, srv, "me")
	if code != 0 {
		t.Fatal("expected success")
	}
	for _, want := range []string{"12345", "Ada Lovelace", "ada@example.com", "Example Co"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}
