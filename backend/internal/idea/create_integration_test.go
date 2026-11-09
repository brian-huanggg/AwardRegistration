//go:build integration

package idea_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"golang.org/x/text/unicode/norm"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
)

// fakeObjects is an in-memory storage.SlideStore. before runs inside Put, so a
// test can change the database between the pre-checks and the transaction.
// deleteErr makes Delete fail (MinIO down after the upload). For Get: onGet
// runs first, getErr fails the open (MinIO down before the first byte), and
// a positive failAfter makes the stream fail with streamErr after that many
// bytes (MinIO down mid-stream).
type fakeObjects struct {
	mu        sync.Mutex
	puts      map[string][]byte
	types     map[string]string
	fail      error
	deleteErr error
	deleted   []string
	last      string // key of the last successful Put
	before    func()

	onGet     func()
	getErr    error
	failAfter int64
	streamErr error
	gets      int
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{puts: map[string][]byte{}, types: map[string]string{}}
}

func (f *fakeObjects) Put(_ context.Context, key string, r io.Reader, size int64, ct string) error {
	if f.before != nil {
		f.before()
	}
	if f.fail != nil {
		return f.fail
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if int64(len(b)) != size {
		return fmt.Errorf("size %d, read %d", size, len(b))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts[key], f.types[key] = b, ct
	f.last = key
	return nil
}

func (f *fakeObjects) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	if f.onGet != nil {
		f.onGet()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	if f.getErr != nil {
		return nil, 0, f.getErr
	}
	b, ok := f.puts[key]
	if !ok {
		return nil, 0, storage.ErrObjectNotFound
	}
	var r io.Reader = bytes.NewReader(b)
	if f.failAfter > 0 {
		r = io.MultiReader(io.LimitReader(r, f.failAfter), failingReader{f.streamErr})
	}
	return io.NopCloser(r), int64(len(b)), nil
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func (f *fakeObjects) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.puts, key)
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *fakeObjects) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.puts)
}

type createEnv struct {
	db      *sql.DB
	objects *fakeObjects
	h       idea.Handler
}

func newCreateEnv(t *testing.T) *createEnv {
	t.Helper()
	d := dbtest.New(t)
	return newCreateEnvOn(t, d, d)
}

// newCreateEnvOn builds the env with the service's transactions on svcDB (for
// example a fault-injecting pool) and every other access on d.
func newCreateEnvOn(t *testing.T, d, svcDB *sql.DB) *createEnv {
	t.Helper()
	aud := audit.New(d, nil)
	objs := newFakeObjects()
	errs := server.ErrorAdapter{Audit: aud}
	e := &createEnv{db: d, objects: objs, h: idea.Handler{
		Service: &idea.Service{
			DB: svcDB, Audit: aud,
			Gate:     deadline.Gate{Store: deadline.SQLStore{DB: d}},
			Slides:   server.IdeaSlideRules(),
			Objects:  objs,
			Aborter:  storage.NewPendingDeletion(d, objs, nil),
			Replaced: server.IdeaReplacedKeys{P: storage.NewPendingDeletion(d, objs, nil)},
		},
		ExpectedFilename: storage.ExpectedFilenameBase,
		Fail:             errs.Write,
	}}
	e.setDeadline(t, "NOW(6) + INTERVAL 1 DAY")
	return e
}

func (e *createEnv) setDeadline(t *testing.T, expr string) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE contest_setting SET deadline = ` + expr + ` WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
}

func (e *createEnv) count(t *testing.T, q string, args ...any) (n int) {
	t.Helper()
	if err := e.db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type form struct {
	fields   [][2]string // in order; a repeated name repeats the part
	filename string
	file     []byte
	noFile   bool
}

func pdf(n int) []byte {
	b := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), n)...)
	return b
}

func pptx(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range []string{"[Content_Types].xml", "ppt/presentation.xml"} {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("<x/>"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// validForm is a valid create for leader 100001 ("User 100001").
func validForm(name string) form {
	return form{
		fields: [][2]string{
			{"category", "COOL_IDEA"}, {"org", "BSID"}, {"direction", "TALENT_DEVELOPMENT"},
			{"name", name}, {"summary", "line one\r\nline two"},
		},
		filename: "I_BSID_User 100001_100001_" + name + ".pdf",
		file:     pdf(100),
	}
}

func (f form) set(name, value string) form {
	out := f
	out.fields = nil
	for _, kv := range f.fields {
		if kv[0] == name {
			kv[1] = value
		}
		out.fields = append(out.fields, kv)
	}
	return out
}

func (f form) body(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, kv := range f.fields {
		if err := mw.WriteField(kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	if !f.noFile {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="slides"; filename="%s"`, f.filename)}
		h["Content-Type"] = []string{"application/octet-stream"}
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(f.file)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func (e *createEnv) post(t *testing.T, p security.Principal, f form) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := f.body(t)
	r := chi.NewRouter()
	r.Post("/ideas", e.h.CreateIdea)
	req := httptest.NewRequest(http.MethodPost, "/ideas", body)
	req.Header.Set("Content-Type", ct)
	req = req.WithContext(security.WithPrincipal(context.Background(), p))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

type problem struct {
	Code   string
	Errors []web.FieldError
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) problem {
	t.Helper()
	var p problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("problem body: %v (%s)", err, rec.Body)
	}
	return p
}

func TestCreateIdeaSuccess(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	f := validForm("Idea A")
	f.fields = append(f.fields, [2]string{"leaderEmpNo", "999999"}) // ignored (PRD 6.6)

	rec := e.post(t, leader, f)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		ID                   string
		Name, Summary        string
		Leader               struct{ EmpNo, Name string }
		PendingRequests      []any
		ExpectedFilenameBase string
		Version              int
		Slides               struct {
			Ext, SHA256, DownloadFilename string
			Size                          int64
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Header().Get("Location") != "/api/v1/ideas/"+body.ID {
		t.Errorf("Location %q, id %s", rec.Header().Get("Location"), body.ID)
	}
	sum := sha256.Sum256(f.file)
	if body.Name != "Idea A" || body.Summary != "line one\nline two" || body.Leader.EmpNo != "100001" ||
		body.Leader.Name != "User 100001" || body.PendingRequests == nil || body.Version != 0 ||
		body.ExpectedFilenameBase != "I_BSID_User 100001_100001_Idea A" ||
		body.Slides.Ext != "pdf" || body.Slides.Size != int64(len(f.file)) || body.Slides.SHA256 != hex.EncodeToString(sum[:]) ||
		body.Slides.DownloadFilename != "I_BSID_User 100001_100001_Idea A.pdf" {
		t.Errorf("body %+v", body)
	}

	var key string
	if err := e.db.QueryRow(`SELECT slides_object_key FROM idea WHERE id = ?`, body.ID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "slides/"+body.ID+"/") || !strings.HasSuffix(key, ".pdf") {
		t.Errorf("object key %q", key)
	}
	if !bytes.Equal(e.objects.puts[key], f.file) || e.objects.types[key] != "application/pdf" {
		t.Errorf("stored object %q: %d bytes, type %q", key, len(e.objects.puts[key]), e.objects.types[key])
	}

	var after string
	if err := e.db.QueryRow(`SELECT after_json FROM audit_log WHERE action = 'IDEA_CREATE' AND outcome = 'SUCCESS'
		AND target_type = 'IDEA' AND target_id = ? AND status = 201 AND route = '/ideas' AND actor_emp_no = '100001'`,
		body.ID).Scan(&after); err != nil {
		t.Fatalf("IDEA_CREATE row: %v", err)
	}
	var a map[string]any
	_ = json.Unmarshal([]byte(after), &a)
	sl, _ := a["slides"].(map[string]any)
	if a["name"] != "Idea A" || a["category"] != "COOL_IDEA" || sl["sha256"] != hex.EncodeToString(sum[:]) || sl["ext"] != "pdf" {
		t.Errorf("after_json %s", after)
	}
}

func TestCreateIdeaPPTXAndNFDFilename(t *testing.T) {
	e := newCreateEnv(t)
	if _, err := e.db.Exec(`INSERT INTO app_user (id, emp_no, name, email, password_hash, role) VALUES (UUID(),'100001',?,?,?, 'USER')`,
		"王小明", "100001@example.com", hash); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := e.db.QueryRow(`SELECT id FROM app_user WHERE emp_no = '100001'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	p := addUserPrincipal(t, id, "100001")
	f := validForm("成立供應商永續學院").set("category", "OUTSTANDING_IMPACT")
	f.filename = norm.NFD.String("O_BSID_王小明_100001_成立供應商永續學院.PPTX")
	f.file = pptx(t)
	rec := e.post(t, p, f)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"ext":"pptx"`) {
		t.Errorf("body %s", rec.Body)
	}
}

func addUserPrincipal(t *testing.T, id, empNo string) security.Principal {
	t.Helper()
	p := security.Principal{EmpNo: empNo, Role: security.RoleUser}
	if err := p.UserID.UnmarshalText([]byte(id)); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCreateIdeaRefusedBeforeBody(t *testing.T) {
	e := newCreateEnv(t)
	user := addUser(t, e.db, "100001", "USER")
	admin := addUser(t, e.db, "900001", "ADMIN")
	broken := form{noFile: true} // would be 400 if the body were read

	rec := e.post(t, admin, broken)
	if rec.Code != http.StatusForbidden || decodeProblem(t, rec).Code != "FORBIDDEN" {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body)
	}
	e.setDeadline(t, "NULL")
	rec = e.post(t, user, broken)
	if rec.Code != http.StatusForbidden || decodeProblem(t, rec).Code != "SUBMISSION_NOT_OPEN" {
		t.Fatalf("not open: %d %s", rec.Code, rec.Body)
	}
	e.setDeadline(t, "NOW(6) - INTERVAL 1 SECOND")
	rec = e.post(t, user, broken)
	if rec.Code != http.StatusForbidden || decodeProblem(t, rec).Code != "SUBMISSION_CLOSED" {
		t.Fatalf("closed: %d %s", rec.Code, rec.Body)
	}
	if got := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_CREATE' AND outcome = 'DENIED' AND status = 403`); got != 3 {
		t.Errorf("DENIED rows = %d, want 3", got)
	}
	if e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_CREATE' AND JSON_VALUE(after_json, '$.reason') = 'ROLE'`) != 1 ||
		e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_CREATE' AND JSON_VALUE(after_json, '$.reason') = 'STATE'`) != 2 {
		t.Error("DENIED reasons: want one ROLE and two STATE")
	}
}

func TestCreateIdeaValidation(t *testing.T) {
	e := newCreateEnv(t)
	user := addUser(t, e.db, "100001", "USER")
	long := strings.Repeat("長", 75) // 225 bytes: valid length, filename too long

	for _, tc := range []struct {
		name   string
		form   form
		status int
		code   string
		fields []web.FieldError
	}{
		{"missing file part", form{fields: validForm("x").fields, noFile: true}, 400, "MALFORMED_REQUEST", nil},
		{"missing text part", func() form { f := validForm("x"); f.fields = f.fields[1:]; return f }(), 400, "MALFORMED_REQUEST", nil},
		{"repeated text part", func() form { f := validForm("x"); f.fields = append(f.fields, f.fields[0]); return f }(), 400, "MALFORMED_REQUEST", nil},
		{
			// Every field reports its first error in form order; the (empty)
			// file is not validated while text is invalid.
			"text errors collected",
			func() form {
				f := validForm("x").set("category", "cool_idea").set("org", " ").set("direction", "GREEN").
					set("name", "a/b").set("summary", "\r\n")
				f.file = nil
				return f
			}(),
			422, "VALIDATION_FAILED",
			[]web.FieldError{
				{Field: "category", Code: "INVALID_ENUM"}, {Field: "org", Code: "REQUIRED"},
				{Field: "direction", Code: "INVALID_ENUM"}, {Field: "name", Code: "INVALID_NAME_CHARS"},
				{Field: "summary", Code: "REQUIRED"},
			},
		},
		{"name chars first", validForm("a:b"), 422, "INVALID_NAME_CHARS", []web.FieldError{{Field: "name", Code: "INVALID_NAME_CHARS"}}},
		{"name too long", validForm(strings.Repeat("a", 76)), 422, "VALIDATION_FAILED", []web.FieldError{{Field: "name", Code: "OUT_OF_RANGE"}}},
		{"summary too long", validForm("x").set("summary", strings.Repeat("s", 301)), 422, "VALIDATION_FAILED", []web.FieldError{{Field: "summary", Code: "OUT_OF_RANGE"}}},
		{"filename too long", validForm(long), 422, "FILENAME_TOO_LONG", []web.FieldError{{Field: "name", Code: "FILENAME_TOO_LONG"}}},
		{"empty file", func() form { f := validForm("x"); f.file = nil; return f }(), 422, "EMPTY_FILE", nil},
		{"over 5 MiB", func() form { f := validForm("x"); f.file = pdf(5 * 1024 * 1024); return f }(), 413, "FILE_TOO_LARGE", nil},
		{"magic mismatch", func() form { f := validForm("x"); f.file = []byte("not a pdf"); return f }(), 415, "UNSUPPORTED_FILE_TYPE", nil},
		{
			"filename mismatch carries expected", func() form { f := validForm("x"); f.filename = "slides.pdf"; return f }(),
			422, "FILENAME_MISMATCH",
			[]web.FieldError{{Field: "slides", Code: "FILENAME_MISMATCH", Expected: "I_BSID_User 100001_100001_x.pdf"}},
		},
		{
			"path is not stripped", func() form { f := validForm("x"); f.filename = "dir/" + f.filename; return f }(),
			422, "FILENAME_MISMATCH",
			[]web.FieldError{{Field: "slides", Code: "FILENAME_MISMATCH", Expected: "I_BSID_User 100001_100001_x.pdf"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := e.post(t, user, tc.form)
			p := decodeProblem(t, rec)
			if rec.Code != tc.status || p.Code != tc.code {
				t.Fatalf("got %d %s, want %d %s (%s)", rec.Code, p.Code, tc.status, tc.code, rec.Body)
			}
			if tc.fields != nil && fmt.Sprint(p.Errors) != fmt.Sprint(tc.fields) {
				t.Errorf("errors %v, want %v", p.Errors, tc.fields)
			}
		})
	}
	if e.objects.count() != 0 || e.count(t, `SELECT COUNT(*) FROM idea`) != 0 {
		t.Error("a rejected create must store nothing")
	}
	if e.count(t, `SELECT COUNT(*) FROM audit_log`) != 0 {
		t.Error("400/413/415/422 are input errors and are not audited")
	}
}

func TestCreateIdeaDuplicate(t *testing.T) {
	e := newCreateEnv(t)
	user := addUser(t, e.db, "100001", "USER")
	other := addUser(t, e.db, "100002", "USER")

	if rec := e.post(t, user, validForm("Same")); rec.Code != 201 {
		t.Fatalf("first: %d %s", rec.Code, rec.Body)
	}
	// Pre-check: 409 and nothing uploaded.
	rec := e.post(t, user, validForm("Same"))
	if rec.Code != 409 || decodeProblem(t, rec).Code != "IDEA_DUPLICATE" || e.objects.count() != 1 {
		t.Fatalf("pre-check: %d %s, %d objects", rec.Code, rec.Body, e.objects.count())
	}
	// Names are case-sensitive and unique per Leader only.
	if rec := e.post(t, user, validForm("same")); rec.Code != 201 {
		t.Errorf("case differs: %d %s", rec.Code, rec.Body)
	}
	f := validForm("Same")
	f.filename = "I_BSID_User 100002_100002_Same.pdf"
	if rec := e.post(t, other, f); rec.Code != 201 {
		t.Errorf("other leader: %d %s", rec.Code, rec.Body)
	}

	// Race past the pre-check: the unique key answers 409 from the transaction.
	e.objects.before = func() {
		e.objects.before = nil
		if err := insert(e.db, newIdea(user.UserID, "Raced")); err != nil {
			t.Error(err)
		}
	}
	rec = e.post(t, user, validForm("Raced"))
	if rec.Code != 409 || decodeProblem(t, rec).Code != "IDEA_DUPLICATE" {
		t.Fatalf("race: %d %s", rec.Code, rec.Body)
	}
	if e.count(t, `SELECT COUNT(*) FROM idea WHERE name = 'Raced'`) != 1 ||
		e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_CREATE' AND outcome = 'SUCCESS'`) != 3 {
		t.Error("race: the losing create must leave no row and no audit row")
	}
	// Known rollback (B3-09): the losing upload is deleted at once, no row kept.
	if len(e.objects.deleted) != 1 || e.objects.count() != 3 || e.pendingCount(t) != 0 {
		t.Errorf("race: deleted %v, %d objects, %d pending rows", e.objects.deleted, e.objects.count(), e.pendingCount(t))
	}
}

func TestCreateIdeaDeadlinePassesBeforeTx(t *testing.T) {
	e := newCreateEnv(t)
	user := addUser(t, e.db, "100001", "USER")
	e.objects.before = func() { e.setDeadline(t, "NOW(6) - INTERVAL 1 SECOND") }

	rec := e.post(t, user, validForm("Late"))
	if rec.Code != 403 || decodeProblem(t, rec).Code != "SUBMISSION_CLOSED" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if e.count(t, `SELECT COUNT(*) FROM idea`) != 0 {
		t.Error("no row may exist")
	}
	if e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_CREATE' AND outcome = 'DENIED'
		AND error_code = 'SUBMISSION_CLOSED' AND JSON_VALUE(after_json, '$.reason') = 'STATE'`) != 1 {
		t.Error("want one DENIED STATE row")
	}
	if len(e.objects.deleted) != 1 || e.objects.count() != 0 || e.pendingCount(t) != 0 {
		t.Errorf("the rolled-back upload must be deleted: deleted %v, %d pending rows", e.objects.deleted, e.pendingCount(t))
	}
}

// Known rollback while MinIO is down: the delete fails, so the key is
// recorded as UPLOAD_ABORTED, due at once; the response is the rollback's.
func TestCreateIdeaRollbackDeleteFails(t *testing.T) {
	e := newCreateEnv(t)
	user := addUser(t, e.db, "100001", "USER")
	e.objects.deleteErr = web.NewError(web.CodeStorageUnavailable, "object storage is unavailable")
	e.objects.before = func() { e.setDeadline(t, "NOW(6) - INTERVAL 1 SECOND") }

	rec := e.post(t, user, validForm("Late"))
	if rec.Code != 403 || decodeProblem(t, rec).Code != "SUBMISSION_CLOSED" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if e.objects.count() != 1 {
		t.Fatalf("%d objects, want the one kept upload", e.objects.count())
	}
	got := e.pending(t, e.objects.last)
	if len(got) != 1 || got[0].reason != "UPLOAD_ABORTED" || got[0].dueInSec > 1 {
		t.Errorf("pending rows %+v, want one UPLOAD_ABORTED due now", got)
	}
}

// Lost-commit cases (Sequence.md §2, §3; Backend.md §4.4) through the
// fault-injecting connector: Unknown keeps the object, records it with the
// grace period and answers 503; a known rollback at COMMIT deletes it.
func TestCreateIdeaLostCommit(t *testing.T) {
	dsn := dbtest.Start(t)
	d := dbtest.Open(t, dsn)
	faulty, faults := dbtest.OpenFaulty(t, dsn)
	e := newCreateEnvOn(t, d, faulty)
	user := addUser(t, d, "100001", "USER")

	for i, tc := range []struct {
		name    string
		fault   dbtest.CommitFault
		applied bool // the idea row exists after the COMMIT
		kept    bool // the object stays, recorded with the grace period
	}{
		{"commit applied, ack lost", dbtest.LostAckCommitted, true, true},
		{"connection lost before commit", dbtest.LostAckNotCommitted, false, true},
		{"deadlock at commit", dbtest.DeadlockAtCommit, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := fmt.Sprintf("Lost %d", i)
			before := len(e.objects.deleted)
			faults.FailNextCommit(tc.fault)
			f := validForm(name)
			rec := e.post(t, user, f)
			if rec.Code != 503 || decodeProblem(t, rec).Code != "SERVICE_UNAVAILABLE" {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			key := e.objects.last
			if n := e.count(t, `SELECT COUNT(*) FROM idea WHERE name = ?`, name); (n == 1) != tc.applied {
				t.Errorf("idea rows = %d, applied %v", n, tc.applied)
			}
			_, stored := e.objects.puts[key]
			got := e.pending(t, key)
			if tc.kept {
				grace := storage.UploadAbortedGrace.Seconds()
				if !stored || len(got) != 1 || got[0].reason != "UPLOAD_ABORTED" ||
					got[0].dueInSec < grace-30 || got[0].dueInSec > grace+1 {
					t.Errorf("Unknown: stored %v, pending %+v; want kept and due after the grace", stored, got)
				}
				return
			}
			if stored || len(got) != 0 || len(e.objects.deleted) != before+1 {
				t.Errorf("rollback: stored %v, pending %+v; want deleted now", stored, got)
			}
		})
	}
	if faults.Fired() != 3 {
		t.Errorf("faults fired = %d, want 3", faults.Fired())
	}
}

type pendingRow struct {
	reason   string
	dueInSec float64 // next_attempt_at - NOW(6), database time
}

func (e *createEnv) pending(t *testing.T, key string) []pendingRow {
	t.Helper()
	rows, err := e.db.Query(`SELECT reason, TIMESTAMPDIFF(MICROSECOND, NOW(6), next_attempt_at) / 1e6
	                           FROM pending_file_deletion WHERE object_key = ? ORDER BY id`, key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.reason, &r.dueInSec); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *createEnv) pendingCount(t *testing.T) int {
	t.Helper()
	return e.count(t, `SELECT COUNT(*) FROM pending_file_deletion`)
}

func TestCreateIdeaStorageDown(t *testing.T) {
	e := newCreateEnv(t)
	user := addUser(t, e.db, "100001", "USER")
	e.objects.fail = web.NewError(web.CodeStorageUnavailable, "object storage is unavailable")

	rec := e.post(t, user, validForm("Idea A"))
	if rec.Code != 503 || decodeProblem(t, rec).Code != "STORAGE_UNAVAILABLE" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if e.count(t, `SELECT COUNT(*) FROM idea`) != 0 || e.count(t, `SELECT COUNT(*) FROM audit_log`) != 0 {
		t.Error("no row and no audit row may exist")
	}
}

// A declared length over the limit is answered before the body is read; the
// 5 MiB + 64 KiB limit leaves room for the text parts of a 5 MiB file.
func TestCreateIdeaDeclaredLengthTooLarge(t *testing.T) {
	e := newCreateEnv(t)
	user := addUser(t, e.db, "100001", "USER")
	body, ct := validForm("x").body(t)
	r := chi.NewRouter()
	r.Post("/ideas", e.h.CreateIdea)
	req := httptest.NewRequest(http.MethodPost, "/ideas", body)
	req.Header.Set("Content-Type", ct)
	req.ContentLength = 5*1024*1024 + 64*1024 + 1
	req = req.WithContext(security.WithPrincipal(context.Background(), user))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 413 || decodeProblem(t, rec).Code != "FILE_TOO_LARGE" || e.objects.count() != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}

	// Exactly 5 MiB of slides still fits.
	f := validForm("Max")
	f.file = pdf(5*1024*1024 - len("%PDF-1.7\n"))
	if rec := e.post(t, user, f); rec.Code != 201 {
		t.Fatalf("5 MiB file: %d %s", rec.Code, rec.Body)
	}
}
