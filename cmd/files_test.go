package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesUpload_TwoStep(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("POST", "/pendingfiles.json", `{"pendingFile":{"ref":"abc123"}}`)
	srv.handle("POST", "/projects/445082/files.json", `{"fileId":"7777","STATUS":"OK"}`)

	tmp := t.TempDir()
	f := filepath.Join(tmp, "report.csv")
	if err := os.WriteFile(f, []byte("col1,col2\n1,2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, srv,
		"files", "upload",
		"--project", "445082",
		"--file", f,
		"--description", "Q2 audit")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "as file 7777") || !strings.Contains(out, "project 445082") {
		t.Errorf("output should reference attached file id, got %q", out)
	}

	// Verify a multipart body landed on /pendingfiles.json…
	var pending, attach *capturedCall
	for i := range srv.calls {
		c := &srv.calls[i]
		switch c.Path {
		case "/pendingfiles.json":
			pending = c
		case "/projects/445082/files.json":
			attach = c
		}
	}
	if pending == nil {
		t.Fatal("pendingfiles.json was never called")
	}
	if !strings.Contains(pending.Body, "report.csv") {
		t.Errorf("pendingfiles body should include filename, got %q", pending.Body)
	}
	if !strings.Contains(pending.Body, "col1,col2") {
		t.Errorf("pendingfiles body should include file bytes, got %q", pending.Body)
	}
	// …and the attach POST included the pending ref + description.
	if attach == nil {
		t.Fatal("/projects/<id>/files.json was never called")
	}
	for _, want := range []string{`"pendingFileRef":"abc123"`, `"description":"Q2 audit"`} {
		if !strings.Contains(attach.Body, want) {
			t.Errorf("attach body missing %q, got %q", want, attach.Body)
		}
	}
}

func TestFilesUpload_RequiresFlags(t *testing.T) {
	srv := newTestServer(t)
	_, errOut, code := runCLI(t, srv, "files", "upload")
	if code == 0 {
		t.Fatal("expected error when flags missing")
	}
	if !strings.Contains(errOut, "--file is required") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestFilesUpload_RequiresExactlyOneTarget(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "tiny.txt")
	_ = os.WriteFile(f, []byte("hi"), 0644)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"neither", []string{"files", "upload", "--file", f}},
		{"both", []string{"files", "upload", "--file", f, "--project", "1", "--task", "42"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t)
			_, errOut, code := runCLI(t, srv, tc.args...)
			if code == 0 {
				t.Fatal("expected error")
			}
			if !strings.Contains(errOut, "exactly one of --project or --task") {
				t.Errorf("stderr = %q", errOut)
			}
		})
	}
}

func TestFilesUpload_AlternateRefShape(t *testing.T) {
	// Newer Teamwork tenants drop the {"pendingFile":{"ref":...}} wrapper and
	// return the ref at the top level. Our extractor should handle both.
	srv := newTestServer(t)
	srv.handle("POST", "/pendingfiles.json", `{"ref":"xyz999"}`)
	srv.handle("POST", "/projects/1/files.json", `{"fileId":"42"}`)

	tmp := t.TempDir()
	f := filepath.Join(tmp, "tiny.txt")
	_ = os.WriteFile(f, []byte("hi"), 0644)

	_, errOut, code := runCLI(t, srv,
		"files", "upload", "--project", "1", "--file", f)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	for _, c := range srv.calls {
		if c.Path == "/projects/1/files.json" && !strings.Contains(c.Body, `"pendingFileRef":"xyz999"`) {
			t.Errorf("attach body should carry top-level ref, got %q", c.Body)
		}
	}
}

func TestFilesUpload_ToTask(t *testing.T) {
	// Attaching to a task is a v1 task update carrying pendingFileAttachments —
	// that is the only parameter that links the file to the task itself.
	srv := newTestServer(t)
	srv.handle("POST", "/pendingfiles.json", `{"pendingFile":{"ref":"tf_abc"}}`)
	srv.handle("GET", "/projects/api/v3/tasks/42.json", `{"task":{"id":42,"status":"new"}}`)
	srv.handle("PUT", "/tasks/42.json", `{"STATUS":"OK","assignedFileIds":["901","902"]}`)

	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.csv")
	b := filepath.Join(tmp, "b.csv")
	_ = os.WriteFile(a, []byte("a"), 0644)
	_ = os.WriteFile(b, []byte("b"), 0644)

	out, errOut, code := runCLI(t, srv, "files", "upload", "--task", "42", "--file", a, "--file", b)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "as file 901 on task 42") || !strings.Contains(out, "as file 902 on task 42") {
		t.Errorf("output should name each attached file id, got %q", out)
	}

	var attach *capturedCall
	pendingCount := 0
	for i := range srv.calls {
		c := &srv.calls[i]
		switch {
		case c.Path == "/pendingfiles.json":
			pendingCount++
		case c.Method == "PUT" && c.Path == "/tasks/42.json":
			attach = c
		case strings.HasSuffix(c.Path, "/uncomplete.json"), strings.HasSuffix(c.Path, "/complete.json"):
			t.Errorf("open task should not be reopened or re-completed, got %s %s", c.Method, c.Path)
		}
	}
	if pendingCount != 2 {
		t.Errorf("expected one pending upload per file, got %d", pendingCount)
	}
	if attach == nil {
		t.Fatal("PUT /tasks/42.json was never called")
	}
	// Both refs ride in a single attach call.
	for _, want := range []string{`"pendingFileAttachments":"tf_abc,tf_abc"`, `"updateFiles":true`} {
		if !strings.Contains(attach.Body, want) {
			t.Errorf("attach body missing %q, got %q", want, attach.Body)
		}
	}
}

func TestFilesUpload_ToCompletedTaskWithoutReopen(t *testing.T) {
	// Teamwork answers a completed-task attach with a bare permission error, so
	// we refuse up front — and before spending the upload.
	srv := newTestServer(t)
	srv.handle("GET", "/projects/api/v3/tasks/42.json", `{"task":{"id":42,"status":"completed"}}`)

	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.csv")
	_ = os.WriteFile(f, []byte("a"), 0644)

	_, errOut, code := runCLI(t, srv, "files", "upload", "--task", "42", "--file", f)
	if code == 0 {
		t.Fatal("expected error on completed task")
	}
	if !strings.Contains(errOut, "is completed") || !strings.Contains(errOut, "--reopen") {
		t.Errorf("stderr should name the cause and the fix, got %q", errOut)
	}
	for _, c := range srv.calls {
		if c.Path == "/pendingfiles.json" {
			t.Error("should not upload bytes before the completed-task check")
		}
	}
}

func TestFilesUpload_ToCompletedTaskWithReopen(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("POST", "/pendingfiles.json", `{"pendingFile":{"ref":"tf_abc"}}`)
	srv.handle("GET", "/projects/api/v3/tasks/42.json", `{"task":{"id":42,"status":"completed"}}`)
	srv.handle("PUT", "/tasks/42/uncomplete.json", `{"STATUS":"OK"}`)
	srv.handle("PUT", "/tasks/42.json", `{"STATUS":"OK","assignedFileIds":["901"]}`)
	srv.handle("PUT", "/tasks/42/complete.json", `{"STATUS":"OK"}`)

	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.csv")
	_ = os.WriteFile(f, []byte("a"), 0644)

	_, errOut, code := runCLI(t, srv, "files", "upload", "--task", "42", "--file", f, "--reopen")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}

	var order []string
	for _, c := range srv.calls {
		switch c.Path {
		case "/tasks/42/uncomplete.json", "/tasks/42.json", "/tasks/42/complete.json":
			order = append(order, c.Path)
		}
	}
	want := []string{"/tasks/42/uncomplete.json", "/tasks/42.json", "/tasks/42/complete.json"}
	if strings.Join(order, " ") != strings.Join(want, " ") {
		t.Errorf("expected reopen → attach → re-complete, got %v", order)
	}
}

func TestFilesUpload_ReCompletesAfterFailedAttach(t *testing.T) {
	// A failed attach must not leave a task we reopened sitting open.
	srv := newTestServer(t)
	srv.handle("POST", "/pendingfiles.json", `{"pendingFile":{"ref":"tf_abc"}}`)
	srv.handle("GET", "/projects/api/v3/tasks/42.json", `{"task":{"id":42,"status":"completed"}}`)
	srv.handle("PUT", "/tasks/42/uncomplete.json", `{"STATUS":"OK"}`)
	srv.handleStatus("PUT", "/tasks/42.json", 500, `{"MESSAGE":"boom","STATUS":"Error"}`)
	srv.handle("PUT", "/tasks/42/complete.json", `{"STATUS":"OK"}`)

	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.csv")
	_ = os.WriteFile(f, []byte("a"), 0644)

	_, _, code := runCLI(t, srv, "files", "upload", "--task", "42", "--file", f, "--reopen")
	if code == 0 {
		t.Fatal("expected a non-zero exit when the attach fails")
	}
	var reCompleted bool
	for _, c := range srv.calls {
		if c.Path == "/tasks/42/complete.json" {
			reCompleted = true
		}
	}
	if !reCompleted {
		t.Error("task should be re-completed even when the attach fails")
	}
}

func TestFilesUpload_TaskRejectsProjectOnlyFlags(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.csv")
	_ = os.WriteFile(f, []byte("a"), 0644)

	for _, extra := range [][]string{{"--description", "hi"}, {"--category", "7"}} {
		srv := newTestServer(t)
		args := append([]string{"files", "upload", "--task", "42", "--file", f}, extra...)
		_, errOut, code := runCLI(t, srv, args...)
		if code == 0 {
			t.Fatalf("expected error for %v", extra)
		}
		if !strings.Contains(errOut, "--project uploads only") {
			t.Errorf("stderr = %q", errOut)
		}
	}
}
