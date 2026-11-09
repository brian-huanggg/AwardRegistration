//go:build integration

package idea_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

// putSlides sends PUT /ideas/{id}/slides with one slides part.
func (e *createEnv) putSlides(t *testing.T, p security.Principal, id uuid.UUID, filename string, file []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, err := mw.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="slides"; filename="%s"`, filename)},
		"Content-Type":        {"application/octet-stream"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(file)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return e.putRaw(p, id, mw.FormDataContentType(), &buf)
}

func (e *createEnv) putRaw(p security.Principal, id uuid.UUID, ct string, body *bytes.Buffer) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Put("/ideas/{id}/slides", func(w http.ResponseWriter, req *http.Request) {
		e.h.ReplaceSlides(w, req, uuid.MustParse(chi.URLParam(req, "id")))
	})
	req := httptest.NewRequest(http.MethodPut, "/ideas/"+id.String()+"/slides", body)
	req.Header.Set("Content-Type", ct)
	req = req.WithContext(security.WithPrincipal(context.Background(), p))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// created makes an Idea through the real create path, so its object exists.
func (e *createEnv) createdIdea(t *testing.T, leader security.Principal, name string) (id uuid.UUID, key string) {
	t.Helper()
	if rec := e.post(t, leader, validForm(name)); rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var sid string
	if err := e.db.QueryRow(`SELECT id, slides_object_key FROM idea WHERE name = ?`, name).Scan(&sid, &key); err != nil {
		t.Fatal(err)
	}
	return uuid.MustParse(sid), key
}

func slidesName(name, ext string) string { return "I_BSID_User 100001_100001_" + name + "." + ext }

func TestReplaceSlidesSuccess(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, oldKey := e.createdIdea(t, leader, "Idea A")
	file := pptx(t)

	rec := e.putSlides(t, leader, id, slidesName("Idea A", "pptx"), file)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	sum := sha256.Sum256(file)
	for _, want := range []string{`"ext":"pptx"`, `"version":1`, `"sha256":"` + hex.EncodeToString(sum[:]) + `"`,
		fmt.Sprintf(`"size":%d`, len(file))} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body lacks %s: %s", want, rec.Body)
		}
	}
	var newKey string
	if err := e.db.QueryRow(`SELECT slides_object_key FROM idea WHERE id = ?`, id.String()).Scan(&newKey); err != nil {
		t.Fatal(err)
	}
	if newKey == oldKey || !strings.HasPrefix(newKey, "slides/"+id.String()+"/") || !strings.HasSuffix(newKey, ".pptx") {
		t.Errorf("new key %q (old %q)", newKey, oldKey)
	}
	if !bytes.Equal(e.objects.puts[newKey], file) || e.objects.types[newKey] == "" {
		t.Error("new object not stored")
	}
	// The old object is gone and its pending row removed after the delete.
	if _, ok := e.objects.puts[oldKey]; ok {
		t.Error("old object still stored")
	}
	if n := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion`); n != 0 {
		t.Errorf("pending rows %d, want 0", n)
	}
	var before, after string
	if err := e.db.QueryRow(`SELECT before_json, after_json FROM audit_log WHERE action = 'SLIDES_REPLACE' AND outcome = 'SUCCESS' AND target_id = ?`,
		id.String()).Scan(&before, &after); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, `"ext":"pdf"`) || !strings.Contains(after, `"ext":"pptx"`) ||
		!strings.Contains(after, `"filename":"`+slidesName("Idea A", "pptx")+`"`) ||
		!strings.Contains(after, hex.EncodeToString(sum[:])) {
		t.Errorf("audit before=%s after=%s", before, after)
	}
}

func TestReplaceSlidesAdminIgnoresDeadline(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	admin := addUser(t, e.db, "900001", "ADMIN")
	id, _ := e.createdIdea(t, leader, "Idea A")
	e.setDeadline(t, "NOW(6) - INTERVAL 1 SECOND")

	if rec := e.putSlides(t, leader, id, slidesName("Idea A", "pdf"), pdf(10)); rec.Code != 403 ||
		decodeProblem(t, rec).Code != "SUBMISSION_CLOSED" {
		t.Fatalf("leader after deadline: %d %s", rec.Code, rec.Body)
	}
	if rec := e.putSlides(t, admin, id, slidesName("Idea A", "pdf"), pdf(10)); rec.Code != 200 {
		t.Fatalf("admin after deadline: %d %s", rec.Code, rec.Body)
	}
}

func TestReplaceSlidesRefusedBeforeBody(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	other := addUser(t, e.db, "100002", "USER")
	id, oldKey := e.createdIdea(t, leader, "Idea A")
	broken := bytes.NewBufferString("not multipart") // would be 400 if the body were read

	check := func(label string, p security.Principal, target uuid.UUID, status int, code string) {
		t.Helper()
		rec := e.putRaw(p, target, "application/json", broken)
		if rec.Code != status || decodeProblem(t, rec).Code != code {
			t.Fatalf("%s: %d %s", label, rec.Code, rec.Body)
		}
	}
	check("hidden", other, id, 404, "NOT_FOUND")
	check("missing", leader, uuid.New(), 404, "NOT_FOUND")
	e.setDeadline(t, "NULL")
	check("not open", leader, id, 403, "SUBMISSION_NOT_OPEN")
	e.setDeadline(t, "NOW(6) - INTERVAL 1 SECOND")
	check("closed", leader, id, 403, "SUBMISSION_CLOSED")

	reasons := func(r string) int {
		return e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_REPLACE' AND outcome = 'DENIED'
		                    AND JSON_VALUE(after_json, '$.reason') = ?`, r)
	}
	if reasons("NOT_FOUND") != 2 || reasons("STATE") != 2 {
		t.Error("DENIED reasons: want 2 NOT_FOUND, 2 STATE")
	}
	var key string
	if err := e.db.QueryRow(`SELECT slides_object_key FROM idea WHERE id = ?`, id.String()).Scan(&key); err != nil || key != oldKey {
		t.Errorf("a refused replace moved the pointer: %q, %v", key, err)
	}
}

func TestReplaceSlidesFileErrors(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, oldKey := e.createdIdea(t, leader, "Idea A")
	objects := e.objects.count()
	tests := []struct {
		label, filename string
		file            []byte
		status          int
		code            string
	}{
		{"over 5 MiB", slidesName("Idea A", "pdf"), pdf(5 * 1024 * 1024), 413, "FILE_TOO_LARGE"},
		{"empty", slidesName("Idea A", "pdf"), nil, 422, "EMPTY_FILE"},
		{"magic mismatch", slidesName("Idea A", "pdf"), []byte("not a pdf"), 415, "UNSUPPORTED_FILE_TYPE"},
		{"filename mismatch", "slides.pdf", pdf(10), 422, "FILENAME_MISMATCH"},
	}
	for _, tc := range tests {
		rec := e.putSlides(t, leader, id, tc.filename, tc.file)
		if rec.Code != tc.status || decodeProblem(t, rec).Code != tc.code {
			t.Errorf("%s: %d %s", tc.label, rec.Code, rec.Body)
		}
	}
	// No rejected request stored an object or moved the pointer.
	var key string
	var version int
	if err := e.db.QueryRow(`SELECT slides_object_key, version FROM idea WHERE id = ?`, id.String()).Scan(&key, &version); err != nil ||
		key != oldKey || version != 0 || e.objects.count() != objects {
		t.Errorf("rejected replaces left traces: %q v%d objects %d (%v)", key, version, e.objects.count(), err)
	}
}

// The expected filename follows the CURRENT Idea data, not the one it was created with.
func TestReplaceSlidesValidatesAgainstCurrentName(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, _ := e.createdIdea(t, leader, "Idea A")
	if rec := e.put(t, leader, id, "application/json", editJSON); rec.Code != 200 { // renames to "Idea B", org ICSD, category O
		t.Fatalf("edit: %d %s", rec.Code, rec.Body)
	}
	if rec := e.putSlides(t, leader, id, slidesName("Idea A", "pdf"), pdf(10)); rec.Code != 422 ||
		decodeProblem(t, rec).Code != "FILENAME_MISMATCH" {
		t.Fatalf("old name: %d %s", rec.Code, rec.Body)
	}
	if rec := e.putSlides(t, leader, id, "O_ICSD_User 100001_100001_Idea B.pdf", pdf(10)); rec.Code != 200 {
		t.Fatalf("new name: %d %s", rec.Code, rec.Body)
	}
}

// MinIO down after the commit: the replace still succeeds and the old key stays
// in pending_file_deletion for the clean-up job.
func TestReplaceSlidesOldDeleteFailureKeepsPendingRow(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, oldKey := e.createdIdea(t, leader, "Idea A")
	e.objects.deleteErr = fmt.Errorf("minio down")

	if rec := e.putSlides(t, leader, id, slidesName("Idea A", "pdf"), pdf(20)); rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion WHERE object_key = ? AND reason = 'REPLACED'`, oldKey); n != 1 {
		t.Errorf("REPLACED rows for the old key: %d, want 1", n)
	}
}

// A failed upload (MinIO down) leaves the row, the audit log and the storage untouched.
func TestReplaceSlidesUploadFailure(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, oldKey := e.createdIdea(t, leader, "Idea A")
	e.objects.fail = fmt.Errorf("minio down")

	if rec := e.putSlides(t, leader, id, slidesName("Idea A", "pdf"), pdf(20)); rec.Code < 500 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var key string
	if err := e.db.QueryRow(`SELECT slides_object_key FROM idea WHERE id = ?`, id.String()).Scan(&key); err != nil || key != oldKey {
		t.Errorf("pointer moved: %q, %v", key, err)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_REPLACE' AND outcome = 'SUCCESS'`); n != 0 {
		t.Errorf("audit rows %d, want 0", n)
	}
}

// The Idea is deleted between the pre-checks and the lock: uniform 404 and the
// uploaded object is removed.
func TestReplaceSlidesRowDeletedMeanwhile(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, _ := e.createdIdea(t, leader, "Idea A")
	e.objects.before = func() {
		if _, err := e.db.Exec(`DELETE FROM idea WHERE id = ?`, id.String()); err != nil {
			t.Error(err)
		}
	}
	rec := e.putSlides(t, leader, id, slidesName("Idea A", "pdf"), pdf(20))
	if rec.Code != 404 || decodeProblem(t, rec).Code != "NOT_FOUND" {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	for k := range e.objects.puts {
		if k != e.objects.last && !strings.HasPrefix(k, "slides/") {
			t.Errorf("unexpected object %s", k)
		}
	}
	if _, ok := e.objects.puts[e.objects.last]; ok && strings.Contains(e.objects.last, id.String()) {
		// last Put is the replacement's new key; it must be deleted by the compensation.
		if len(e.objects.deleted) == 0 {
			t.Error("new object not deleted after a known rollback")
		}
	}
}

// Concurrent replaces: every one succeeds, the version counts them, and exactly
// one object stays referenced; every other key is deleted or pending REPLACED.
func TestReplaceSlidesConcurrentLeavesNoOrphan(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, _ := e.createdIdea(t, leader, "Idea A")
	const n = 8
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := e.putSlides(t, leader, id, slidesName("Idea A", "pdf"), pdf(10+i)); rec.Code != 200 {
				t.Errorf("replace %d: %d %s", i, rec.Code, rec.Body)
			}
		}()
	}
	wg.Wait()

	var key string
	var version int
	if err := e.db.QueryRow(`SELECT slides_object_key, version FROM idea WHERE id = ?`, id.String()).Scan(&key, &version); err != nil {
		t.Fatal(err)
	}
	if version != n {
		t.Errorf("version %d, want %d", version, n)
	}
	if _, ok := e.objects.puts[key]; !ok || e.objects.count() != 1 {
		t.Errorf("stored objects %d, want exactly the referenced %q", e.objects.count(), key)
	}
	if got := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion`); got != 0 {
		t.Errorf("pending rows %d, want 0 (every old key deleted)", got)
	}
	if got := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_REPLACE' AND outcome = 'SUCCESS'`); got != n {
		t.Errorf("audit rows %d, want %d", got, n)
	}
}
