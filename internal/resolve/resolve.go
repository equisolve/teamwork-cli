// Package resolve maps human-friendly names to Teamwork numeric IDs with a
// tiny on-disk cache keyed on (kind, query).
package resolve

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/equisolve/teamwork-cli/internal/api"
)

// Client is the subset of api.Client we use, extracted so tests can inject
// fakes without spinning up an HTTP server.
type Client interface {
	Get(path string, params url.Values) (json.RawMessage, error)
}

type entry struct {
	ID int   `json:"id"`
	TS int64 `json:"ts"`
}

type cacheData struct {
	Me        entry            `json:"me,omitempty"`
	Projects  map[string]entry `json:"projects,omitempty"`
	People    map[string]entry `json:"people,omitempty"`
	Companies map[string]entry `json:"companies,omitempty"`
}

// Resolver handles name→ID translation with disk caching.
type Resolver struct {
	Client Client
	// CachePath defaults to ~/.config/teamwork/cache.json.
	CachePath string
}

func New(client Client) *Resolver {
	return &Resolver{Client: client, CachePath: defaultCachePath()}
}

func defaultCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "teamwork", "cache.json")
}

func (r *Resolver) load() *cacheData {
	c := &cacheData{}
	data, err := os.ReadFile(r.CachePath)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, c)
	return c
}

func (r *Resolver) save(c *cacheData) error {
	if err := os.MkdirAll(filepath.Dir(r.CachePath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.CachePath, data, 0600)
}

// Clear removes the cache file.
func (r *Resolver) Clear() error {
	err := os.Remove(r.CachePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Me returns the authenticated user's person id. Cached indefinitely.
func (r *Resolver) Me() (int, error) {
	cache := r.load()
	if cache.Me.ID != 0 {
		return cache.Me.ID, nil
	}

	body, err := r.Client.Get("/me.json", nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Person struct {
			ID json.Number `json:"id"`
		} `json:"person"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("could not parse /me.json: %w", err)
	}
	id, err := resp.Person.ID.Int64()
	if err != nil || id == 0 {
		return 0, fmt.Errorf("could not determine person id from /me.json")
	}
	cache.Me = entry{ID: int(id), TS: time.Now().Unix()}
	_ = r.save(cache)
	return int(id), nil
}

// Person resolves a numeric ID, the literal "me", or a search string to a
// person id.
func (r *Resolver) Person(query string) (int, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, fmt.Errorf("empty person query")
	}
	if id, err := strconv.Atoi(q); err == nil {
		return id, nil
	}
	if strings.EqualFold(q, "me") {
		return r.Me()
	}
	return r.resolveBy("people", q, "/people.json", "people", "id")
}

// Project resolves a numeric ID or a project name search string to a project id.
func (r *Resolver) Project(query string) (int, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, fmt.Errorf("empty project query")
	}
	if id, err := strconv.Atoi(q); err == nil {
		return id, nil
	}
	return r.resolveBy("projects", q, "/projects.json", "projects", "id")
}

// Company resolves a numeric ID or a company name search string to a company id.
func (r *Resolver) Company(query string) (int, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, fmt.Errorf("empty company query")
	}
	if id, err := strconv.Atoi(q); err == nil {
		return id, nil
	}
	return r.resolveBy("companies", q, "/companies.json", "companies", "id")
}

// resolveBy hits a v1 listing endpoint with ?searchTerm=q and expects exactly
// one matching result. Caches successful resolutions.
func (r *Resolver) resolveBy(kind, query, path, arrayKey, idKey string) (int, error) {
	cache := r.load()
	cacheKey := strings.ToLower(query)

	bucket := cacheBucket(cache, kind)
	if e, ok := bucket[cacheKey]; ok && e.ID != 0 {
		return e.ID, nil
	}

	params := url.Values{}
	params.Set("searchTerm", query)
	params.Set("pageSize", "25")

	body, err := r.Client.Get(path, params)
	if err != nil {
		return 0, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, fmt.Errorf("could not parse %s: %w", path, err)
	}
	arr, ok := raw[arrayKey]
	if !ok {
		return 0, fmt.Errorf("unexpected response from %s (no %q field)", path, arrayKey)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(arr, &items); err != nil {
		return 0, fmt.Errorf("could not parse %s items: %w", path, err)
	}

	matches := filterMatches(items, query, kind)
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no %s matches %q", kind, query)
	case 1:
		id, err := extractID(matches[0], idKey)
		if err != nil {
			return 0, err
		}
		bucket[cacheKey] = entry{ID: id, TS: time.Now().Unix()}
		setCacheBucket(cache, kind, bucket)
		_ = r.save(cache)
		return id, nil
	default:
		return 0, ambiguousError(kind, query, matches)
	}
}

func cacheBucket(c *cacheData, kind string) map[string]entry {
	switch kind {
	case "projects":
		if c.Projects == nil {
			c.Projects = map[string]entry{}
		}
		return c.Projects
	case "people":
		if c.People == nil {
			c.People = map[string]entry{}
		}
		return c.People
	case "companies":
		if c.Companies == nil {
			c.Companies = map[string]entry{}
		}
		return c.Companies
	}
	return map[string]entry{}
}

func setCacheBucket(c *cacheData, kind string, bucket map[string]entry) {
	switch kind {
	case "projects":
		c.Projects = bucket
	case "people":
		c.People = bucket
	case "companies":
		c.Companies = bucket
	}
}

// filterMatches narrows the result set to rows whose display name contains the
// query string case-insensitively. Teamwork's searchTerm can be fuzzy; we want
// exact-ish matches before declaring ambiguity.
func filterMatches(items []map[string]json.RawMessage, query, kind string) []map[string]json.RawMessage {
	q := strings.ToLower(query)
	var out []map[string]json.RawMessage
	for _, it := range items {
		if containsField(it, q, nameFields(kind)...) {
			out = append(out, it)
		}
	}
	// If no strict filter match, fall back to the full set so the caller can
	// surface an ambiguity error with the actual response.
	if len(out) == 0 {
		return items
	}
	return out
}

func nameFields(kind string) []string {
	switch kind {
	case "people":
		return []string{"first-name", "last-name", "email-address", "full-name"}
	case "projects", "companies":
		return []string{"name"}
	}
	return []string{"name"}
}

func containsField(item map[string]json.RawMessage, needle string, fields ...string) bool {
	for _, f := range fields {
		raw, ok := item[f]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(s), needle) {
			return true
		}
	}
	return false
}

func extractID(item map[string]json.RawMessage, key string) (int, error) {
	raw, ok := item[key]
	if !ok {
		return 0, fmt.Errorf("item has no %q field", key)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			n = json.Number(s)
		} else {
			return 0, fmt.Errorf("could not parse id: %w", err)
		}
	}
	i, err := n.Int64()
	if err != nil {
		return 0, fmt.Errorf("could not parse id: %w", err)
	}
	return int(i), nil
}

func ambiguousError(kind, query string, matches []map[string]json.RawMessage) error {
	preview := make([]string, 0, len(matches))
	for i, m := range matches {
		if i >= 5 {
			preview = append(preview, fmt.Sprintf("… and %d more", len(matches)-5))
			break
		}
		id, _ := extractID(m, "id")
		name := firstStringField(m, "name", "full-name")
		if name == "" {
			name = firstStringField(m, "first-name", "last-name")
		}
		preview = append(preview, fmt.Sprintf("%d: %s", id, name))
	}
	return &api.APIError{
		StatusCode: 0,
		Message: fmt.Sprintf("ambiguous %s query %q — %d matches:\n  %s",
			kind, query, len(matches), strings.Join(preview, "\n  ")),
	}
}

func firstStringField(item map[string]json.RawMessage, fields ...string) string {
	for _, f := range fields {
		raw, ok := item[f]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}
