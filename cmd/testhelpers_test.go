package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func resetFlagsRecursive(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
	for _, sub := range c.Commands() {
		resetFlagsRecursive(sub)
	}
}

// testRoute matches method+path and returns a canned body.
type testRoute struct {
	method string
	path   string
	body   string
	status int
}

// testServer is a tiny router for our httptest-based command tests. Routes are
// matched by method+exact path (query string ignored).
type testServer struct {
	mu     sync.Mutex
	routes []testRoute
	calls  []capturedCall
	srv    *httptest.Server
}

type capturedCall struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ts := &testServer{}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.mu.Lock()
		defer ts.mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		ts.calls = append(ts.calls, capturedCall{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body),
		})
		for _, rt := range ts.routes {
			if rt.method == r.Method && rt.path == r.URL.Path {
				if rt.status != 0 {
					w.WriteHeader(rt.status)
				}
				fmt.Fprint(w, rt.body)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"MESSAGE":"route not mocked: `+r.Method+" "+r.URL.Path+`"}`)
	}))
	t.Cleanup(ts.srv.Close)
	return ts
}

func (ts *testServer) handle(method, path, body string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.routes = append(ts.routes, testRoute{method: method, path: path, body: body})
}

func (ts *testServer) handleStatus(method, path string, status int, body string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.routes = append(ts.routes, testRoute{method: method, path: path, body: body, status: status})
}

func (ts *testServer) url() string { return ts.srv.URL }

// exitPanic carries the exit code out of exitFn via panic so tests can recover.
type exitPanic int

// runCLI invokes the root cobra command with the given args, with stdout/stderr
// captured and config isolated to a temp HOME. Returns stdout, stderr, and the
// exit code (0 if the command returned normally).
func runCLI(t *testing.T, srv *testServer, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TEAMWORK_URL", srv.url())
	t.Setenv("TEAMWORK_TOKEN", "testtoken")

	// Reset package-level state between runs.
	urlFlag = ""
	tokenFlag = ""
	outputFlag = "table"
	resolverCache = nil
	_ = rootCmd.PersistentFlags().Set("json", "false")
	_ = rootCmd.PersistentFlags().Set("output", "table")

	// Cobra keeps flag state between Execute calls — reset every flag back to
	// its declared default so each test starts clean.
	resetFlagsRecursive(rootCmd)

	// Replace exitFn with a panicker we can recover.
	oldExit := exitFn
	exitFn = func(c int) { panic(exitPanic(c)) }
	defer func() { exitFn = oldExit }()

	// Capture stdout/stderr via pipes.
	oldStdout, oldStderr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout = outW
	os.Stderr = errW
	defer func() { os.Stdout = oldStdout; os.Stderr = oldStderr }()

	rootCmd.SetArgs(args)
	rootCmd.SetOut(outW)
	rootCmd.SetErr(errW)

	code = 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ec, ok := r.(exitPanic); ok {
					code = int(ec)
					return
				}
				panic(r)
			}
		}()
		if err := rootCmd.Execute(); err != nil {
			code = 1
		}
	}()

	outW.Close()
	errW.Close()
	var outBuf, errBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, outR)
	_, _ = io.Copy(&errBuf, errR)

	return outBuf.String(), errBuf.String(), code
}

// jsonField extracts a JSON path like "projects.0.id" from raw output.
func jsonField(t *testing.T, raw string, path string) string {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, raw)
	}
	for _, part := range strings.Split(path, ".") {
		switch node := v.(type) {
		case map[string]interface{}:
			v = node[part]
		case []interface{}:
			var i int
			fmt.Sscanf(part, "%d", &i)
			if i >= len(node) {
				return ""
			}
			v = node[i]
		default:
			return ""
		}
	}
	return fmt.Sprintf("%v", v)
}
