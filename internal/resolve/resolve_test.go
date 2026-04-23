package resolve

import (
	"encoding/json"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

type fakeClient struct {
	resp  map[string]json.RawMessage
	calls []string
	err   error
}

func (f *fakeClient) Get(path string, params url.Values) (json.RawMessage, error) {
	f.calls = append(f.calls, path+"?"+params.Encode())
	if f.err != nil {
		return nil, f.err
	}
	if body, ok := f.resp[path]; ok {
		return body, nil
	}
	return json.RawMessage(`{}`), nil
}

func tempResolver(t *testing.T, fc *fakeClient) *Resolver {
	t.Helper()
	return &Resolver{
		Client:    fc,
		CachePath: filepath.Join(t.TempDir(), "cache.json"),
	}
}

func TestPerson_NumericPassthrough(t *testing.T) {
	r := tempResolver(t, &fakeClient{})
	id, err := r.Person("42")
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Errorf("got %d, want 42", id)
	}
}

func TestPerson_Me_CachesAfterFirstCall(t *testing.T) {
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/me.json": json.RawMessage(`{"person":{"id":"86714"}}`),
	}}
	r := tempResolver(t, fc)

	id, err := r.Person("me")
	if err != nil {
		t.Fatal(err)
	}
	if id != 86714 {
		t.Errorf("got %d, want 86714", id)
	}
	// second call should hit cache
	fc.resp = map[string]json.RawMessage{} // clear; cache-only should still work
	id2, err := r.Person("me")
	if err != nil {
		t.Fatal(err)
	}
	if id2 != 86714 {
		t.Errorf("second call got %d, want 86714", id2)
	}
}

func TestPerson_Me_CachePersistsAcrossInstances(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/me.json": json.RawMessage(`{"person":{"id":"99"}}`),
	}}
	r1 := &Resolver{Client: fc, CachePath: cachePath}
	if _, err := r1.Me(); err != nil {
		t.Fatal(err)
	}

	fc2 := &fakeClient{err: errors.New("should not be called")}
	r2 := &Resolver{Client: fc2, CachePath: cachePath}
	id, err := r2.Me()
	if err != nil {
		t.Fatal(err)
	}
	if id != 99 {
		t.Errorf("got %d, want 99", id)
	}
	if len(fc2.calls) != 0 {
		t.Errorf("expected no calls on second resolver, got %v", fc2.calls)
	}
}

func TestProject_SearchSingleMatch(t *testing.T) {
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/projects.json": json.RawMessage(`{"projects":[{"id":"795529","name":"Acme Corp"}]}`),
	}}
	r := tempResolver(t, fc)
	id, err := r.Project("Acme")
	if err != nil {
		t.Fatal(err)
	}
	if id != 795529 {
		t.Errorf("got %d, want 795529", id)
	}
}

func TestProject_SearchNoMatch(t *testing.T) {
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/projects.json": json.RawMessage(`{"projects":[]}`),
	}}
	r := tempResolver(t, fc)
	_, err := r.Project("Zzzzz")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no projects matches") {
		t.Errorf("error = %q, want 'no projects matches'", err.Error())
	}
}

func TestProject_SearchAmbiguous(t *testing.T) {
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/projects.json": json.RawMessage(`{"projects":[
			{"id":"1","name":"Acme Widgets"},
			{"id":"2","name":"Acme Sprockets"}
		]}`),
	}}
	r := tempResolver(t, fc)
	_, err := r.Project("Acme")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Acme Widgets") {
		t.Errorf("expected preview to include match names: %q", err.Error())
	}
}

func TestPerson_SearchByEmail(t *testing.T) {
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/people.json": json.RawMessage(`{"people":[
			{"id":"42","first-name":"Ada","last-name":"Lovelace","email-address":"ada@example.com"}
		]}`),
	}}
	r := tempResolver(t, fc)
	id, err := r.Person("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Errorf("got %d, want 42", id)
	}
}

func TestCompany_CachesResolution(t *testing.T) {
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/companies.json": json.RawMessage(`{"companies":[{"id":"43901","name":"Example Co"}]}`),
	}}
	r := tempResolver(t, fc)
	if _, err := r.Company("Example Co"); err != nil {
		t.Fatal(err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("first resolve: calls = %d, want 1", len(fc.calls))
	}
	// second call should be cached, no new HTTP call.
	if _, err := r.Company("Example Co"); err != nil {
		t.Fatal(err)
	}
	if len(fc.calls) != 1 {
		t.Errorf("second resolve issued a new call: calls = %d", len(fc.calls))
	}
}

func TestClear_RemovesCache(t *testing.T) {
	fc := &fakeClient{resp: map[string]json.RawMessage{
		"/me.json": json.RawMessage(`{"person":{"id":"1"}}`),
	}}
	r := tempResolver(t, fc)
	if _, err := r.Me(); err != nil {
		t.Fatal(err)
	}
	if err := r.Clear(); err != nil {
		t.Fatal(err)
	}
	// Clear on non-existent file should also be ok.
	if err := r.Clear(); err != nil {
		t.Fatal(err)
	}
}

func TestPerson_Empty(t *testing.T) {
	r := tempResolver(t, &fakeClient{})
	if _, err := r.Person(""); err == nil {
		t.Error("expected error for empty query")
	}
}
