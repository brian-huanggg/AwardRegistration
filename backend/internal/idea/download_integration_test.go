//go:build integration

package idea_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// created posts a valid create as p and returns the new Idea's ID and the key
// of its stored object.
func (e *createEnv) created(t *testing.T, p security.Principal, name string) (uuid.UUID, string) {
	t.Helper()
	return e.createdWith(t, p, validForm(name))
}

func (e *createEnv) createdWith(t *testing.T, p security.Principal, f form) (uuid.UUID, string) {
	t.Helper()
	rec := e.post(t, p, f)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var b struct{ ID uuid.UUID }
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b.ID, e.objects.last
}

func (e *createEnv) downloadRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/ideas/{id}/slides", func(w http.ResponseWriter, req *http.Request) {
		id, err := uuid.Parse(chi.URLParam(req, "id"))
		if err != nil {
			panic(err)
		}
		e.h.DownloadSlides(w, req, api.IdeaId(id))
	})
	return r
}

func (e *createEnv) download(t *testing.T, p security.Principal, id uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ideas/"+id.String()+"/slides", nil)
	req = req.WithContext(security.WithPrincipal(context.Background(), p))
	rec := httptest.NewRecorder()
	e.downloadRouter().ServeHTTP(rec, req)
	return rec
}

func (e *createEnv) gets() int {
	e.objects.mu.Lock()
	defer e.objects.mu.Unlock()
	return e.objects.gets
}

func TestDownloadSlides(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	other := addUser(t, e.db, "100002", "USER")
	admin := addUser(t, e.db, "900001", "ADMIN")
	id, key := e.created(t, leader, "Idea A")
	successRows := func() int {
		return e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_DOWNLOAD' AND outcome = 'SUCCESS'`)
	}

	// The audit row is committed (visible to another connection) before the
	// object is even opened, so before the first byte.
	rowsAtOpen := -1
	e.objects.onGet = func() { rowsAtOpen = successRows() }

	rec := e.download(t, leader, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("leader: %d %s", rec.Code, rec.Body)
	}
	if rowsAtOpen != 1 {
		t.Errorf("SLIDES_DOWNLOAD rows when the object was opened = %d, want 1", rowsAtOpen)
	}
	if !bytes.Equal(rec.Body.Bytes(), e.objects.puts[key]) {
		t.Error("body differs from the stored object")
	}
	want := map[string]string{
		"Content-Type":           "application/pdf",
		"Content-Length":         strconv.Itoa(len(e.objects.puts[key])),
		"Content-Disposition":    idea.ContentDisposition("I_BSID_User 100001_100001_Idea A.pdf"),
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if rec.Header().Get("Accept-Ranges") != "" {
		t.Error("Range must not be advertised")
	}
	if n := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_DOWNLOAD' AND outcome = 'SUCCESS'
	        AND actor_role = 'USER' AND actor_emp_no = '100001' AND target_type = 'IDEA' AND target_id = ?
	        AND method = 'GET' AND route = '/ideas/{id}/slides' AND status = 200`, id.String()); n != 1 {
		t.Errorf("leader SUCCESS row with full fields = %d, want 1", n)
	}

	// Admin: 200 and a second SUCCESS row.
	if rec := e.download(t, admin, id); rec.Code != http.StatusOK {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_DOWNLOAD' AND outcome = 'SUCCESS' AND actor_role = 'ADMIN'`); n != 1 {
		t.Errorf("admin SUCCESS rows = %d, want 1", n)
	}

	// Hidden and missing: identical 404 (bar the request ID), both DENIED, and
	// storage is never touched.
	before := e.gets()
	hidden, missing := e.download(t, other, id), e.download(t, other, uuid.New())
	if hidden.Code != 404 || missing.Code != 404 {
		t.Fatalf("codes %d %d", hidden.Code, missing.Code)
	}
	strip := func(rr *httptest.ResponseRecorder) map[string]any {
		var m map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &m)
		delete(m, "requestId")
		return m
	}
	hb, mb := strip(hidden), strip(missing)
	if len(hb) != len(mb) || hb["code"] != "NOT_FOUND" || hb["code"] != mb["code"] || hb["detail"] != mb["detail"] {
		t.Errorf("bodies differ: %v vs %v", hb, mb)
	}
	for k := range hidden.Header() {
		if hidden.Header().Get(k) != missing.Header().Get(k) {
			t.Errorf("header %s differs", k)
		}
	}
	if e.gets() != before {
		t.Error("a 404 must not open the object")
	}
	if n := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_DOWNLOAD' AND outcome = 'DENIED'
	        AND status = 404 AND error_code = 'NOT_FOUND'`); n != 2 {
		t.Errorf("DENIED rows = %d, want 2", n)
	}
	if successRows() != 2 {
		t.Errorf("SUCCESS rows = %d, want 2", successRows())
	}

	// Not state-limited, and the name is the *current* expected filename.
	e.setDeadline(t, "NOW(6) - INTERVAL 1 DAY")
	if _, err := e.db.Exec(`UPDATE idea SET name = 'Renamed' WHERE id = ?`, id.String()); err != nil {
		t.Fatal(err)
	}
	rec = e.download(t, leader, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("after deadline: %d %s", rec.Code, rec.Body)
	}
	if got, want := rec.Header().Get("Content-Disposition"), idea.ContentDisposition("I_BSID_User 100001_100001_Renamed.pdf"); got != want {
		t.Errorf("Content-Disposition = %q, want %q", got, want)
	}
}

// A non-ASCII Idea name is served exactly through filename*.
func TestDownloadSlidesUnicodeFilename(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, _ := e.created(t, leader, "減碳 Idea")
	rec := e.download(t, leader, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	want := `attachment; filename="I_BSID_User 100001_100001___ Idea.pdf"; ` +
		`filename*=UTF-8''I_BSID_User%20100001_100001_%E6%B8%9B%E7%A2%B3%20Idea.pdf`
	if got := rec.Header().Get("Content-Disposition"); got != want {
		t.Errorf("Content-Disposition = %q, want %q", got, want)
	}
}

// Failures before the first byte answer a problem; the audit row stays
// (Sequence.md §6: the intent to download was real).
func TestDownloadSlidesFailsBeforeFirstByte(t *testing.T) {
	for _, tc := range []struct {
		name    string
		breakIt func(e *createEnv, id uuid.UUID, key string)
		status  int
		code    string
	}{
		{"MinIO down", func(e *createEnv, _ uuid.UUID, _ string) {
			e.objects.getErr = web.NewError(web.CodeStorageUnavailable, "object storage is unavailable")
		}, 503, "STORAGE_UNAVAILABLE"},
		{"object missing", func(e *createEnv, _ uuid.UUID, key string) {
			delete(e.objects.puts, key)
		}, 500, "INTERNAL_ERROR"},
		{"object length differs from the row", func(e *createEnv, id uuid.UUID, _ string) {
			if _, err := e.db.Exec(`UPDATE idea SET slides_size = slides_size + 1 WHERE id = ?`, id.String()); err != nil {
				panic(err)
			}
		}, 500, "INTERNAL_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newCreateEnv(t)
			leader := addUser(t, e.db, "100001", "USER")
			id, key := e.created(t, leader, "Idea A")
			tc.breakIt(e, id, key)
			rec := e.download(t, leader, id)
			if rec.Code != tc.status || decodeProblem(t, rec).Code != tc.code {
				t.Fatalf("%d %s, want %d %s", rec.Code, rec.Body, tc.status, tc.code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != web.ProblemContentType {
				t.Errorf("Content-Type = %q", ct)
			}
			if n := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'SLIDES_DOWNLOAD' AND outcome = 'SUCCESS'`); n != 1 {
				t.Errorf("SUCCESS rows = %d, want 1", n)
			}
		})
	}
}

// A storage failure mid-stream aborts the connection, so the client never
// takes a short body for the whole file. Past net/http's write buffer the 200
// and its Content-Length are already out and the body ends early; before it,
// the client gets no response at all.
func TestDownloadSlidesAbortsMidStream(t *testing.T) {
	const size = 64 << 10
	for _, tc := range []struct {
		name      string
		failAfter int64
		headers   bool // the 200 reached the client
	}{
		{"after the headers were sent", 32 << 10, true},
		{"within the write buffer", 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newCreateEnv(t)
			leader := addUser(t, e.db, "100001", "USER")
			f := validForm("Idea A")
			f.file = pdf(size)
			id, key := e.createdWith(t, leader, f)
			total := int64(len(e.objects.puts[key]))
			e.objects.failAfter = tc.failAfter
			e.objects.streamErr = errors.New("minio: connection reset")

			inner := e.downloadRouter()
			srv := httptest.NewServer(web.Recover(slog.New(slog.DiscardHandler))(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					inner.ServeHTTP(w, r.WithContext(security.WithPrincipal(r.Context(), leader)))
				})))
			t.Cleanup(srv.Close)

			resp, err := http.Get(srv.URL + "/ideas/" + id.String() + "/slides")
			if !tc.headers {
				if err == nil {
					_ = resp.Body.Close()
					t.Fatalf("got a response (%d); want the connection aborted", resp.StatusCode)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != 200 || resp.ContentLength != total {
				t.Fatalf("status %d, length %d, want 200 %d", resp.StatusCode, resp.ContentLength, total)
			}
			b, err := io.ReadAll(resp.Body)
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("read error = %v, want unexpected EOF", err)
			}
			if int64(len(b)) >= total {
				t.Errorf("read %d of %d bytes; want a short body", len(b), total)
			}
		})
	}
}

// An unknown commit outcome serves nothing: the audit row may not exist.
func TestDownloadSlidesUnknownOutcome(t *testing.T) {
	dsn := dbtest.Start(t)
	d := dbtest.Open(t, dsn)
	faulty, faults := dbtest.OpenFaulty(t, dsn)
	e := newCreateEnvOn(t, d, faulty)
	leader := addUser(t, d, "100001", "USER")
	id, _ := e.created(t, leader, "Idea A")

	for _, fault := range []dbtest.CommitFault{dbtest.LostAckCommitted, dbtest.LostAckNotCommitted} {
		before := e.gets()
		faults.FailNextCommit(fault)
		rec := e.download(t, leader, id)
		if rec.Code != 503 || decodeProblem(t, rec).Code != "SERVICE_UNAVAILABLE" {
			t.Fatalf("fault %v: %d %s", fault, rec.Code, rec.Body)
		}
		if e.gets() != before {
			t.Errorf("fault %v: the object must not be opened", fault)
		}
	}
	if faults.Fired() != 2 {
		t.Errorf("faults fired = %d, want 2", faults.Fired())
	}
}
