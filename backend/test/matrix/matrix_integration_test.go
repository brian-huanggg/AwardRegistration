//go:build integration

package matrix

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	shareddb "github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
	"github.com/brian-huanggg/AwardRegistration/backend/test/apispec"
)

const password = "Correct-Horse-1"

type env struct {
	db      *sql.DB
	plain   http.Handler // production chain
	checked http.Handler // same chain, every response validated against openapi.yaml
	seq     atomic.Int64
}

// newEnv wires the same chain as cmd/esg: this harness must fail if the
// middleware order in production changes, so it must not build its own.
func newEnv(t *testing.T) *env {
	t.Helper()
	db := dbtest.New(t)
	log := slog.New(slog.DiscardHandler)
	auditSvc := audit.New(db, log)
	errs := server.ErrorAdapter{Audit: auditSvc, Log: log}
	csrf, err := auth.NewCSRF(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.New(db)
	logins, err := auth.NewLogins(sessions, auditSvc, auth.NoThrottle{})
	if err != nil {
		t.Fatal(err)
	}
	si := server.API{
		Auth:    auth.Handler{CSRF: csrf, Log: log},
		Logins:  auth.LoginHandler{Logins: logins, Fail: errs.Write},
		Logouts: auth.LogoutHandler{Logouts: &auth.Logouts{Sessions: sessions, Audit: auditSvc}, Fail: errs.Write},
		Me:      auth.MeHandler{Sessions: sessions, Fail: errs.Write},
		Account: account.Handler{
			Service: &account.Service{DB: db, Audit: auditSvc, Policy: account.DefaultPolicy()},
			Fail:    errs.Write,
		},
		Deadline: deadline.Handler{Store: deadline.SQLStore{DB: db}},
		Ideas: idea.Handler{
			Service: &idea.Service{
				DB: db, Audit: auditSvc,
				Gate:     deadline.Gate{Store: deadline.SQLStore{DB: db}},
				Slides:   server.IdeaSlideRules(),
				Objects:  discardObjects{},
				Aborter:  discardObjects{},
				Replaced: discardObjects{},
			},
			ExpectedFilename: storage.ExpectedFilenameBase,
			Fail:             errs.Write,
		},
	}
	chain := server.NewPublic(log, server.Mount(si),
		auth.Middleware(sessions, server.IsPublic, log),
		csrf.Middleware(errs.Write, server.ActionFor))
	return &env{db: db, plain: chain, checked: apispec.Load(t).Validate(t, chain)}
}

// discardObjects stands in for MinIO: the matrix is about who may call what,
// not about storage.
type discardObjects struct{}

func (discardObjects) Put(_ context.Context, _ string, r io.Reader, _ int64, _ string) error {
	_, err := io.Copy(io.Discard, r)
	return err
}

// Get is never reached by the matrix rows (they download only missing Ideas).
func (discardObjects) Get(context.Context, string) (io.ReadCloser, int64, error) {
	return nil, 0, errors.New("matrix: no objects")
}

func (discardObjects) AbortUpload(context.Context, string, shareddb.Outcome) {}

func (discardObjects) RecordReplaced(context.Context, *sql.Tx, string) (int64, error) { return 0, nil }

func (discardObjects) DeleteAfterCommit(context.Context, int64, string) {}

// stateDeadline is the deadline expression that puts the contest in a state.
var stateDeadline = map[string]string{
	"NOT_OPEN": "NULL",
	"OPEN":     "NOW(6) + INTERVAL 1 DAY",
	"CLOSED":   "NOW(6) - INTERVAL 1 DAY",
}

func (e *env) setState(t *testing.T, state string) {
	t.Helper()
	expr, ok := stateDeadline[state]
	if !ok {
		t.Fatalf("unknown state %q", state)
	}
	if _, err := e.db.Exec(`UPDATE contest_setting SET deadline = ` + expr + ` WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
}

func (e *env) do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func (e *env) addUser(t *testing.T, empNo, role string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		t.Fatal(err)
	}
	var email any = empNo + "@example.com"
	if role == "ADMIN" {
		email = nil
	}
	if _, err := e.db.Exec(`INSERT INTO app_user (id, emp_no, name, email, password_hash, role, enabled) VALUES (?,?,?,?,?,?,?)`,
		uuid.NewString(), empNo, "N "+empNo, email, string(hash), role, true); err != nil {
		t.Fatal(err)
	}
}

// actor is one caller of the matrix: its employee number, account name and,
// when logged in, its session cookie.
type actor struct {
	empNo   string
	name    string
	session *http.Cookie
	leader  bool // leads the Idea that ${ideaId} stands for
}

// fixtures lists the actors the harness can build. The leader is a User who
// leads the Idea that ${ideaId} stands for; the member arrives with B5-01.
var fixtures = map[string]string{"visitor": "", "user": "USER", "leader": "USER", "admin": "ADMIN"}

// newActor creates a fresh account (logout and login change session state, so
// no two cells share one) and logs in through the API.
func (e *env) newActor(t *testing.T, name string) actor {
	t.Helper()
	role, ok := fixtures[name]
	if !ok {
		t.Fatalf("matrix actor %q has no fixture; add one before adding cells for it", name)
	}
	if role == "" {
		return actor{empNo: "000000"}
	}
	empNo := fmt.Sprintf("%06d", 100000+e.seq.Add(1))
	e.addUser(t, empNo, role)
	tok, ck := e.csrf(t, nil)
	body := fmt.Sprintf(`{"empNo":%q,"password":%q}`, empNo, password)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeader, tok)
	req.AddCookie(ck)
	rec := e.do(e.plain, req)
	if rec.Code != 200 {
		t.Fatalf("login %s: %d %s", name, rec.Code, rec.Body)
	}
	return actor{empNo: empNo, name: "N " + empNo, session: cookie(rec, auth.SessionCookie), leader: name == "leader"}
}

func cookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (e *env) csrf(t *testing.T, session *http.Cookie) (string, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/auth/csrf", nil)
	if session != nil {
		req.AddCookie(session)
	}
	rec := e.do(e.plain, req)
	var b struct{ Token string }
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil || rec.Code != 200 {
		t.Fatalf("csrf: %d %s", rec.Code, rec.Body)
	}
	return b.Token, cookie(rec, auth.CSRFCookie)
}

// seedIdea inserts an Idea for ${ideaId}. It is led by the actor when the actor
// is the leader, otherwise by a fresh User, so every cell has its own Idea.
func (e *env) seedIdea(t *testing.T, a actor) (id, leaderEmpNo string) {
	t.Helper()
	empNo := a.empNo
	if !a.leader {
		empNo = fmt.Sprintf("%06d", 700000+e.seq.Add(1))
		e.addUser(t, empNo, "USER")
	}
	var leaderID string
	if err := e.db.QueryRow(`SELECT id FROM app_user WHERE emp_no = ?`, empNo).Scan(&leaderID); err != nil {
		t.Fatal(err)
	}
	id = uuid.NewString()
	if _, err := e.db.Exec(`INSERT INTO idea (id, leader_id, category, org, direction, name, summary,
	        slides_object_key, slides_ext, slides_size, slides_sha256)
	        VALUES (?, ?, 'COOL_IDEA', 'BSID', 'TALENT_DEVELOPMENT', 'Matrix Seed', 'Seed summary', ?, 'pdf', 1024, ?)`,
		id, leaderID, "slides/"+id+"/"+uuid.NewString()+".pdf", strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}
	return id, empNo
}

func (e *env) request(t *testing.T, r Row, a actor) *http.Request {
	t.Helper()
	var seeded string // employee number of the seeded Idea's Leader
	if strings.Contains(r.Path, "${ideaId}") {
		var id string
		id, seeded = e.seedIdea(t, a)
		r.Path = strings.ReplaceAll(r.Path, "${ideaId}", id)
	}
	var body *strings.Reader
	if r.Body != nil {
		s := string(r.Body)
		s = strings.ReplaceAll(s, "${password}", password)
		s = strings.ReplaceAll(s, "${actorEmpNo}", a.empNo)
		s = strings.ReplaceAll(s, "${newEmpNo}", fmt.Sprintf("%06d", 500000+e.seq.Add(1)))
		body = strings.NewReader(s)
	}
	var req *http.Request
	if r.Multipart != nil {
		buf, ct := multipartBody(t, r.Multipart, a, seeded)
		req = httptest.NewRequest(r.Method, r.Path, buf)
		req.Header.Set("Content-Type", ct)
	} else if body != nil {
		req = httptest.NewRequest(r.Method, r.Path, body)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(r.Method, r.Path, nil)
	}
	req.RemoteAddr = "192.0.2.9:4000"
	req.Header.Set("User-Agent", "matrix/1")
	if a.session != nil {
		req.AddCookie(a.session)
	}
	if r.Method != http.MethodGet && r.CSRF != "none" {
		tok, ck := e.csrf(t, a.session)
		req.Header.Set(auth.CSRFHeader, tok)
		req.AddCookie(ck)
	}
	return req
}

func multipartBody(t *testing.T, m *Multipart, a actor, leaderEmpNo string) (*bytes.Buffer, string) {
	t.Helper()
	// ${leaderName}/${leaderEmpNo} name the Leader of the seeded ${ideaId} Idea.
	sub := strings.NewReplacer("${actorEmpNo}", a.empNo, "${actorName}", a.name,
		"${leaderEmpNo}", leaderEmpNo, "${leaderName}", "N "+leaderEmpNo)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range m.Fields {
		if err := mw.WriteField(k, sub.Replace(v)); err != nil {
			t.Fatal(err)
		}
	}
	w, err := mw.CreateFormFile(m.File.Field, sub.Replace(m.File.Filename))
	if err != nil {
		t.Fatal(err)
	}
	switch m.File.Content {
	case "pdf":
		_, _ = w.Write([]byte("%PDF-1.7\nmatrix\n%%EOF\n"))
	default:
		t.Fatalf("unknown file content %q", m.File.Content)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func TestPermissionMatrix(t *testing.T) {
	e := newEnv(t)
	for _, r := range load(t) {
		if r.State != "" {
			e.setState(t, r.State)
		}
		for _, name := range Actors {
			want, ok := r.Expect[name]
			if !ok {
				continue
			}
			t.Run(r.ID+"/"+name, func(t *testing.T) {
				a := e.newActor(t, name)
				h := e.checked
				if r.OutsideSpec {
					h = e.plain
				}
				rec := e.do(h, e.request(t, r, a))
				if rec.Code != want.Status {
					t.Fatalf("%s %s as %s: status %d, want %d (%s)", r.Method, r.Path, name, rec.Code, want.Status, rec.Body)
				}
				if want.Code != "" {
					var p struct{ Code string }
					if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != want.Code {
						t.Fatalf("%s %s as %s: code %q, want %q (%s)", r.Method, r.Path, name, p.Code, want.Code, rec.Body)
					}
				}
			})
		}
	}
}
