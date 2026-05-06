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
	if !strings.Contains(errOut, "--project and --file are required") {
		t.Errorf("stderr = %q", errOut)
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
