package cmd

import (
	"strings"
	"testing"
)

func TestCompaniesList_Table(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/companies.json", `{
		"companies": [
			{"id": 43901, "name": "Example Co", "website": "http://example.com/",
			 "countryCode": "US", "updatedAt": "2017-05-15T00:00:00Z"}
		],
		"meta": {"page": {"count": 1}}
	}`)

	out, _, code := runCLI(t, srv, "companies", "list")
	if code != 0 {
		t.Fatal("expected success")
	}
	if !strings.Contains(out, "Example Co") || !strings.Contains(out, "2017-05-15") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestCompaniesShow_ByName(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/companies.json", `{"companies":[{"id":"43901","name":"Example Co"}]}`)
	srv.handle("GET", "/projects/api/v3/companies/43901.json", `{
		"company": {"id": 43901, "name": "Example Co", "website": "http://example.com/",
		            "countryCode": "US"}
	}`)

	out, _, code := runCLI(t, srv, "companies", "show", "Example Co")
	if code != 0 {
		t.Fatal("expected success")
	}
	if !strings.Contains(out, "Example Co") || !strings.Contains(out, "http://example.com/") {
		t.Errorf("unexpected output:\n%s", out)
	}
}
