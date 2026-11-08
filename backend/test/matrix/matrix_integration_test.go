//go:build integration

package matrix

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
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
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
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
			Service: &account.Service{DB: db, Audit: auditSvc, Policy: account.StubPolicy{}},
			Fail:    errs.Write,
		},
		Deadline: deadline.Handler{Store: deadline.SQLStore{DB: db}},
	}
	chain := server.NewPublic(log, server.Mount(si),
		auth.Middleware(sessions, server.IsPublic, log),
		csrf.Middleware(errs.Write, server.ActionFor))
	return &env{db: db, plain: chain, checked: apispec.Load(t).Validate(t, chain)}
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

// actor is one caller of the matrix: its employee number and, when logged in,
// its session cookie.
type actor struct {
	empNo   string
	session *http.Cookie
}

// fixtures lists the actors the harness can build. Idea-related actors (member,
// leader) arrive with the PRs that create Ideas.
var fixtures = map[string]string{"visitor": "", "user": "USER", "admin": "ADMIN"}

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
	return actor{empNo: empNo, session: cookie(rec, auth.SessionCookie)}
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

func (e *env) request(t *testing.T, r Row, a actor) *http.Request {
	t.Helper()
	var body *strings.Reader
	if r.Body != nil {
		s := string(r.Body)
		s = strings.ReplaceAll(s, "${password}", password)
		s = strings.ReplaceAll(s, "${actorEmpNo}", a.empNo)
		s = strings.ReplaceAll(s, "${newEmpNo}", fmt.Sprintf("%06d", 500000+e.seq.Add(1)))
		body = strings.NewReader(s)
	}
	var req *http.Request
	if body != nil {
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

func TestPermissionMatrix(t *testing.T) {
	e := newEnv(t)
	for _, r := range load(t) {
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
