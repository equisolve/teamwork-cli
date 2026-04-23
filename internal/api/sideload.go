package api

import "encoding/json"

// V3Included models the `included` object returned by Teamwork v3 responses
// when sideloading with `?include=...`. Keys are entity types (e.g. "companies",
// "projects", "users", "tags") and values map string IDs to raw entity JSON.
type V3Included map[string]map[string]json.RawMessage

// ParseIncluded extracts the v3 `included` object from a response body. Returns
// an empty (non-nil) map if absent or unparseable.
func ParseIncluded(body json.RawMessage) V3Included {
	var wrap struct {
		Included V3Included `json:"included"`
	}
	_ = json.Unmarshal(body, &wrap)
	if wrap.Included == nil {
		return V3Included{}
	}
	return wrap.Included
}

// Lookup returns the raw entity JSON for a given type+id, or nil if absent.
func (i V3Included) Lookup(kind, id string) json.RawMessage {
	if i == nil {
		return nil
	}
	bucket, ok := i[kind]
	if !ok {
		return nil
	}
	return bucket[id]
}

// LookupString reads a string field from an included entity. Empty string if
// the entity or field is missing.
func (i V3Included) LookupString(kind, id, field string) string {
	raw := i.Lookup(kind, id)
	if raw == nil {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	v, ok := m[field]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return s
}
