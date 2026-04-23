package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "testtoken"), srv
}

func TestGet_BasicAuthHeader(t *testing.T) {
	var gotAuth string
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"ok":true}`))
	})
	if _, err := c.Get("/foo", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("testtoken:x"))
	if gotAuth != want {
		t.Errorf("auth header = %q, want %q", gotAuth, want)
	}
}

func TestGet_PathAndParams(t *testing.T) {
	var gotURL string
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Write([]byte(`{}`))
	})
	p := url.Values{}
	p.Set("page", "3")
	p.Set("status", "active")
	if _, err := c.Get("/projects.json", p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(gotURL, "/projects.json?") {
		t.Fatalf("url = %q, want /projects.json? prefix", gotURL)
	}
	if !strings.Contains(gotURL, "page=3") || !strings.Contains(gotURL, "status=active") {
		t.Errorf("url = %q missing expected params", gotURL)
	}
}

func TestPost_SerializesJSON(t *testing.T) {
	var gotBody []byte
	var gotContentType string
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"id":"42"}`))
	})
	payload := map[string]string{"name": "Test"}
	if _, err := c.Post("/tasks.json", nil, payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	var got map[string]string
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if got["name"] != "Test" {
		t.Errorf("body = %v, want name=Test", got)
	}
}

func TestDelete_NoBody(t *testing.T) {
	var gotMethod string
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := c.Delete("/tasks/1.json", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
}

func TestExtractError_Shapes(t *testing.T) {
	cases := map[string]string{
		`{"MESSAGE":"bad token"}`:                 "bad token",
		`{"error":"not allowed"}`:                 "not allowed",
		`{"errors":[{"message":"first bad"}]}`:    "first bad",
		`{"unrelated":"field"}`:                   "",
		`not json`:                                "",
	}
	for in, want := range cases {
		if got := extractError([]byte(in)); got != want {
			t.Errorf("extractError(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDo_ReturnsAPIErrorOnNon2xx(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"MESSAGE":"Task not found"}`))
	})
	_, err := c.Get("/tasks/1.json", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err is %T, want *APIError", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("status = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "Task not found" {
		t.Errorf("message = %q", apiErr.Message)
	}
}

func TestFormatError_StatusMessages(t *testing.T) {
	cases := []struct {
		code int
		msg  string
		want string
	}{
		{401, "anything", "Invalid API token"},
		{403, "anything", "does not have permission"},
		{404, "anything", "Not found."},
		{422, "bad field", "Request rejected: bad field"},
		{429, "anything", "Rate limited"},
		{500, "oops", "oops"},
	}
	for _, tc := range cases {
		err := &APIError{StatusCode: tc.code, Message: tc.msg}
		got := FormatError(err, "https://example.teamwork.com")
		if !strings.Contains(got, tc.want) {
			t.Errorf("FormatError(%d,%q) = %q, want to contain %q", tc.code, tc.msg, got, tc.want)
		}
	}
}

func TestFormatError_NonAPIError(t *testing.T) {
	got := FormatError(fmt.Errorf("connection refused"), "")
	if got != "connection refused" {
		t.Errorf("got %q", got)
	}
}

func TestPaginateV1_StopsWhenPageUnderfull(t *testing.T) {
	calls := 0
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			w.Write([]byte(`{"projects":[{"id":"1"},{"id":"2"}]}`)) // full
		case "2":
			w.Write([]byte(`{"projects":[{"id":"3"}]}`)) // underfull → stop
		default:
			t.Errorf("unexpected page %q", page)
			w.Write([]byte(`{"projects":[]}`))
		}
	})

	var total int
	err := c.PaginateV1("/projects.json", nil, 2, func(body json.RawMessage, n int) (bool, error) {
		total += n
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if total != 3 {
		t.Errorf("total items = %d, want 3", total)
	}
}

func TestPaginateV1_CallbackStops(t *testing.T) {
	calls := 0
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"projects":[{"id":"1"},{"id":"2"}]}`))
	})
	err := c.PaginateV1("/projects.json", nil, 2, func(body json.RawMessage, n int) (bool, error) {
		return false, nil // stop after first page
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestPaginateV3_FollowsHasMore(t *testing.T) {
	calls := 0
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			w.Write([]byte(`{"projects":[{"id":"1"}],"meta":{"page":{"hasMore":true}}}`))
		case "2":
			w.Write([]byte(`{"projects":[{"id":"2"}],"meta":{"page":{"hasMore":false}}}`))
		default:
			t.Errorf("unexpected page %q", page)
			w.Write([]byte(`{}`))
		}
	})
	var pages int
	err := c.PaginateV3("/projects/api/v3/projects.json", nil, func(raw json.RawMessage) (bool, error) {
		pages++
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Errorf("pages = %d, want 2", pages)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestCountFirstArray(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{`{"projects":[{"id":"1"},{"id":"2"}]}`, 2},
		{`{"tasks":[]}`, 0},
		{`{"scalar":"ignored","list":[1,2,3]}`, 3},
		{`not json`, 0},
	}
	for _, tc := range cases {
		if got := countFirstArray([]byte(tc.body)); got != tc.want {
			t.Errorf("countFirstArray(%q) = %d, want %d", tc.body, got, tc.want)
		}
	}
}

func TestV3HasMore(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{`{"meta":{"page":{"hasMore":true}}}`, true},
		{`{"meta":{"page":{"hasMore":false}}}`, false},
		{`{}`, false},
		{`not json`, false},
	}
	for _, tc := range cases {
		if got := v3HasMore([]byte(tc.body)); got != tc.want {
			t.Errorf("v3HasMore(%q) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

func TestParseIncluded(t *testing.T) {
	body := []byte(`{
		"projects": [{"id":"1"}],
		"included": {
			"companies": {"10":{"id":"10","name":"Acme"}},
			"users":     {"99":{"id":"99","firstName":"Eric"}}
		}
	}`)
	inc := ParseIncluded(body)
	if got := inc.LookupString("companies", "10", "name"); got != "Acme" {
		t.Errorf("company 10 name = %q, want Acme", got)
	}
	if got := inc.LookupString("users", "99", "firstName"); got != "Eric" {
		t.Errorf("user 99 firstName = %q, want Eric", got)
	}
	if got := inc.LookupString("companies", "404", "name"); got != "" {
		t.Errorf("missing company = %q, want empty", got)
	}
}

func TestParseIncluded_Missing(t *testing.T) {
	inc := ParseIncluded([]byte(`{"projects":[]}`))
	if inc == nil {
		t.Fatal("expected non-nil map")
	}
	if got := inc.LookupString("x", "y", "z"); got != "" {
		t.Errorf("got %q", got)
	}
}
