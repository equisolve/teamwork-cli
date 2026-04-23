package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// do builds a request, applies basic auth (apikey:x), and returns the raw body
// on 2xx, or an APIError otherwise.
func (c *Client) do(method, path string, params url.Values, body io.Reader) (json.RawMessage, error) {
	u := c.BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.SetBasicAuth(c.Token, "x")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := extractError(data)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	return json.RawMessage(data), nil
}

func extractError(body []byte) string {
	var s struct {
		Message string `json:"MESSAGE"`
		Error   string `json:"error"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &s) == nil {
		if s.Message != "" {
			return s.Message
		}
		if s.Error != "" {
			return s.Error
		}
		if len(s.Errors) > 0 && s.Errors[0].Message != "" {
			return s.Errors[0].Message
		}
	}
	return ""
}

func (c *Client) Get(path string, params url.Values) (json.RawMessage, error) {
	return c.do("GET", path, params, nil)
}

func (c *Client) Post(path string, params url.Values, payload interface{}) (json.RawMessage, error) {
	return c.do("POST", path, params, marshalBody(payload))
}

func (c *Client) Put(path string, params url.Values, payload interface{}) (json.RawMessage, error) {
	return c.do("PUT", path, params, marshalBody(payload))
}

func (c *Client) Delete(path string, params url.Values) (json.RawMessage, error) {
	return c.do("DELETE", path, params, nil)
}

func marshalBody(payload interface{}) io.Reader {
	if payload == nil {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return bytes.NewReader(b)
}

// PaginateV1 iterates pages of a v1 endpoint using page=N&pageSize=M and stops
// when a response returns fewer items than pageSize. The onPage callback
// receives each raw page body and returns (more, err); if more=false iteration
// stops. pageSize defaults to 250 (v1 cap is 500 but many endpoints are less).
func (c *Client) PaginateV1(path string, params url.Values, pageSize int, onPage func(raw json.RawMessage, itemCount int) (more bool, err error)) error {
	if pageSize == 0 {
		pageSize = 250
	}
	page := 1
	for {
		p := cloneValues(params)
		p.Set("page", fmt.Sprintf("%d", page))
		p.Set("pageSize", fmt.Sprintf("%d", pageSize))

		body, err := c.Get(path, p)
		if err != nil {
			return err
		}
		count := countFirstArray(body)
		more, err := onPage(body, count)
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
		if count < pageSize {
			return nil
		}
		page++
	}
}

// PaginateV3 iterates pages of a v3 endpoint following the meta.page.hasMore
// object returned by Teamwork v3 responses.
func (c *Client) PaginateV3(path string, params url.Values, onPage func(raw json.RawMessage) (more bool, err error)) error {
	page := 1
	for {
		p := cloneValues(params)
		p.Set("page", fmt.Sprintf("%d", page))

		body, err := c.Get(path, p)
		if err != nil {
			return err
		}
		more, err := onPage(body)
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
		if !v3HasMore(body) {
			return nil
		}
		page++
	}
}

func cloneValues(in url.Values) url.Values {
	out := url.Values{}
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// countFirstArray returns the length of the first array field found in a v1
// response body (e.g. "projects", "tasks", "people"). Used by PaginateV1 to
// decide if the page was full.
func countFirstArray(body json.RawMessage) int {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return 0
	}
	for _, v := range m {
		if len(v) == 0 {
			continue
		}
		if v[0] != '[' {
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(v, &arr); err == nil {
			return len(arr)
		}
	}
	return 0
}

func v3HasMore(body json.RawMessage) bool {
	var wrap struct {
		Meta struct {
			Page struct {
				HasMore bool `json:"hasMore"`
			} `json:"page"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return false
	}
	return wrap.Meta.Page.HasMore
}

// FormatError returns a user-friendly error message.
func FormatError(err error, baseURL string) string {
	if apiErr, ok := err.(*APIError); ok {
		switch apiErr.StatusCode {
		case 401:
			return fmt.Sprintf("Invalid API token. Generate a new one at %s (Profile → Edit My Profile → API & Mobile).", baseURL)
		case 403:
			return "Your user account does not have permission for this action."
		case 404:
			return "Not found."
		case 422:
			return "Request rejected: " + apiErr.Message
		case 429:
			return "Rate limited by Teamwork. Try again in a moment."
		default:
			return apiErr.Message
		}
	}
	return err.Error()
}
