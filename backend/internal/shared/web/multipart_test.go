package web

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mpPart struct{ name, filename, body string }

func mpRequest(t *testing.T, parts []mpPart) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		disp := `form-data; name="` + p.name + `"`
		if p.filename != "" {
			disp += `; filename="` + p.filename + `"`
		}
		pw, err := w.CreatePart(map[string][]string{"Content-Disposition": {disp}})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = pw.Write([]byte(p.body))
	}
	_ = w.Close()
	r := httptest.NewRequest("POST", "/ideas", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}

func spec(dir string) MultipartSpec {
	return MultipartSpec{FileField: "slides", TextFields: []string{"name"}, TooLarge: CodeFileTooLarge, TempDir: dir}
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	if es, _ := os.ReadDir(dir); len(es) != 0 {
		t.Fatalf("temp dir not empty: %v", es)
	}
}

func wantCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func TestReadMultipartSuccess(t *testing.T) {
	dir := t.TempDir()
	r := mpRequest(t, []mpPart{{"name", "", "Idea"}, {"extra", "", "ignored"}, {"slides", `a/b\c.pdf`, "%PDF-1"}})
	f, err := ReadMultipart(r, spec(dir))
	if err != nil {
		t.Fatal(err)
	}
	if f.Fields["name"] != "Idea" || f.File.Size != 6 || f.File.Filename != `a/b\c.pdf` {
		t.Fatalf("form = %+v file = %+v", f.Fields, f.File)
	}
	if f.File.SHA256 != sha256.Sum256([]byte("%PDF-1")) {
		t.Fatal("sha256 mismatch")
	}
	b, _ := io.ReadAll(f.File.Open())
	head := make([]byte, 5)
	_, _ = f.File.ReaderAt().ReadAt(head, 0)
	if string(b) != "%PDF-1" || string(head) != "%PDF-" {
		t.Fatalf("content %q head %q", b, head)
	}
	if es, _ := os.ReadDir(dir); len(es) != 1 {
		t.Fatalf("want one temp file while open, got %d", len(es))
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal("second Close must be a no-op:", err)
	}
	assertEmpty(t, dir)
}

func TestReadMultipartMalformedRemovesTempFile(t *testing.T) {
	tests := []struct {
		name  string
		parts []mpPart
	}{
		{"missing file", []mpPart{{"name", "", "x"}}},
		{"missing text", []mpPart{{"slides", "a.pdf", "x"}}},
		{"duplicate file", []mpPart{{"name", "", "x"}, {"slides", "a.pdf", "x"}, {"slides", "b.pdf", "y"}}},
		{"duplicate text", []mpPart{{"name", "", "x"}, {"name", "", "y"}, {"slides", "a.pdf", "x"}}},
		{"file without filename", []mpPart{{"name", "", "x"}, {"slides", "", "x"}}},
		{"non-UTF-8 filename", []mpPart{{"name", "", "x"}, {"slides", "\xff.pdf", "x"}}},
		{"non-UTF-8 text", []mpPart{{"name", "", "\xff"}, {"slides", "a.pdf", "x"}}},
		{"oversized text", []mpPart{{"name", "", strings.Repeat("a", maxTextPartBytes+1)}, {"slides", "a.pdf", "x"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			f, err := ReadMultipart(mpRequest(t, tc.parts), spec(dir))
			if f != nil {
				t.Fatal("form must be nil on error")
			}
			wantCode(t, err, CodeMalformedRequest)
			assertEmpty(t, dir)
		})
	}
}

func TestReadMultipartNotMultipart(t *testing.T) {
	r := httptest.NewRequest("POST", "/ideas", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	_, err := ReadMultipart(r, spec(t.TempDir()))
	wantCode(t, err, CodeMalformedRequest)
}

func TestReadMultipartTruncatedBody(t *testing.T) {
	dir := t.TempDir()
	r := mpRequest(t, []mpPart{{"name", "", "x"}, {"slides", "a.pdf", strings.Repeat("a", 1000)}})
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body[:len(body)/2]))
	_, err := ReadMultipart(r, spec(dir))
	wantCode(t, err, CodeMalformedRequest)
	assertEmpty(t, dir)
}

func TestReadMultipartBodyLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parts []mpPart
	}{
		{"inside file part", []mpPart{{"name", "", "x"}, {"slides", "a.pdf", strings.Repeat("a", 4000)}}},
		{"inside ignored part", []mpPart{{"junk", "", strings.Repeat("a", 4000)}, {"name", "", "x"}, {"slides", "a.pdf", "x"}}},
		{"inside text part", []mpPart{{"name", "", strings.Repeat("a", 4000)}, {"slides", "a.pdf", "x"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			r := mpRequest(t, tc.parts)
			rec := httptest.NewRecorder()
			r.Body = http.MaxBytesReader(rec, r.Body, 1024)
			_, err := ReadMultipart(r, spec(dir))
			wantCode(t, err, CodeFileTooLarge)
			assertEmpty(t, dir)
		})
	}
}

func TestReadMultipartTempDirUnwritable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := ReadMultipart(mpRequest(t, []mpPart{{"name", "", "x"}, {"slides", "a.pdf", "x"}}), spec(missing))
	var e *Error
	if err == nil || errors.As(err, &e) {
		t.Fatalf("want plain (500) error, got %v", err)
	}
}

func TestFormCloseNil(t *testing.T) {
	var f *Form
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
