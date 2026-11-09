//go:build integration

package auth_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/test/apispec"
)

const password = "Correct-Horse-1"

type fakeThrottle struct {
	mu                 sync.Mutex
	retry              time.Duration
	err                error
	checks, fails, oks int
	lastKey            string
}

func (f *fakeThrottle) Check(_ context.Context, key string, _ netip.Addr) (time.Duration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	f.lastKey = key
	return f.retry, f.err
}
func (f *fakeThrottle) AuditOnce(context.Context, string, netip.Addr) (bool, error) {
	return true, nil
}
func (f *fakeThrottle) Fail(context.Context, string, netip.Addr) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fails++
	return nil
}
func (f *fakeThrottle) Succeed(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oks++
	return nil
}

type loginEnv struct {
	db *sql.DB
	h  http.Handler
	th *fakeThrottle
}

func newLoginEnv(t *testing.T) loginEnv {
	t.Helper()
	return newLoginEnvLog(t, slog.New(slog.DiscardHandler))
}

func newLoginEnvLog(t *testing.T, log *slog.Logger) loginEnv {
	t.Helper()
	return newLoginEnvWith(t, log, nil)
}

// newLoginEnvWith wires mk(db) as the throttle; nil keeps the fake.
func newLoginEnvWith(t *testing.T, log *slog.Logger, mk func(*sql.DB) auth.Throttle) loginEnv {
	t.Helper()
	db := dbtest.New(t)
	auditSvc := audit.New(db, nil)
	errs := server.ErrorAdapter{Audit: auditSvc}
	sessions := auth.New(db)
	th := &fakeThrottle{}
	var thr auth.Throttle = th
	if mk != nil {
		thr = mk(db)
	}
	logins, err := auth.NewLogins(sessions, auditSvc, thr)
	if err != nil {
		t.Fatal(err)
	}
	csrf, err := auth.NewCSRF(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	si := server.API{
		Auth:    auth.Handler{CSRF: csrf, Log: log},
		Logins:  auth.LoginHandler{Logins: logins, Fail: errs.Write},
		Logouts: auth.LogoutHandler{Logouts: &auth.Logouts{Sessions: sessions, Audit: auditSvc}, Fail: errs.Write},
		Me:      auth.MeHandler{Sessions: sessions, Fail: errs.Write},
	}
	chain := server.NewPublic(log, server.Mount(si),
		auth.Middleware(sessions, server.IsPublic, log),
		csrf.Middleware(errs.Write, server.ActionFor))
	return loginEnv{db: db, h: apispec.Load(t).Validate(t, chain), th: th}
}

func (e loginEnv) addUser(t *testing.T, empNo, role string, enabled bool) uuid.UUID {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	var email any = empNo + "@example.com"
	if role == "ADMIN" {
		email = nil
	}
	if _, err := e.db.Exec(`INSERT INTO app_user (id, emp_no, name, email, password_hash, role, enabled) VALUES (?,?,?,?,?,?,?)`,
		id.String(), empNo, "N "+empNo, email, string(hash), role, enabled); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e loginEnv) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

// csrf fetches a token the way the client does, optionally with a session.
func (e loginEnv) csrf(t *testing.T, session *http.Cookie) (token string, cookie *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/auth/csrf", nil)
	if session != nil {
		req.AddCookie(session)
	}
	rec := e.do(req)
	var b struct{ Token string }
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil || rec.Code != 200 {
		t.Fatalf("csrf: %d %s", rec.Code, rec.Body)
	}
	return b.Token, findCookie(rec, auth.CSRFCookie)
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (e loginEnv) login(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	tok, ck := e.csrf(t, nil)
	return e.loginWith(body, tok, ck)
}

func (e loginEnv) loginWith(body, tok string, ck *http.Cookie, extra ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", tok)
	req.Header.Set("User-Agent", "it/1")
	req.RemoteAddr = "192.0.2.9:4000"
	req.AddCookie(ck)
	for _, c := range extra {
		req.AddCookie(c)
	}
	return e.do(req)
}

func creds(empNo, pw string) string {
	b, _ := json.Marshal(map[string]string{"empNo": empNo, "password": pw})
	return string(b)
}

type row struct {
	Action, Outcome, Actor, ErrorCode string
	Empno, Submitted, After           sql.NullString
	Status                            sql.NullInt64
}

func (e loginEnv) audits(t *testing.T) []row {
	t.Helper()
	rs, err := e.db.Query(`SELECT action, outcome, actor_role, COALESCE(error_code,''), actor_emp_no,
		submitted_emp_no, CAST(after_json AS CHAR), status FROM audit_log WHERE action='AUTH_LOGIN' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.Action, &r.Outcome, &r.Actor, &r.ErrorCode, &r.Empno, &r.Submitted, &r.After, &r.Status); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func (e loginEnv) sessions(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM session`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func code(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Code
}

func TestLoginSuccess(t *testing.T) {
	e := newLoginEnv(t)
	id := e.addUser(t, "123456", "USER", true)

	rec := e.login(t, creds("123456", password))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body) != 4 || body["empNo"] != "123456" || body["name"] != "N 123456" ||
		body["email"] != "123456@example.com" || body["role"] != "USER" {
		t.Errorf("body = %v", body)
	}
	c := findCookie(rec, auth.SessionCookie)
	if c == nil || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" {
		t.Fatalf("session cookie = %+v", c)
	}
	if _, ok := auth.Binding(c.Value); !ok {
		t.Errorf("cookie value %q is not a session ID", c.Value)
	}
	if e.sessions(t) != 1 {
		t.Error("want exactly one session row")
	}

	rows := e.audits(t)
	if len(rows) != 1 {
		t.Fatalf("audit = %+v", rows)
	}
	r := rows[0]
	if r.Outcome != "SUCCESS" || r.Actor != "USER" || r.Empno.String != "123456" || r.Status.Int64 != 200 || r.Submitted.Valid {
		t.Errorf("audit row = %+v", r)
	}
	var target string
	if err := e.db.QueryRow(`SELECT target_id FROM audit_log WHERE action='AUTH_LOGIN'`).Scan(&target); err != nil || target != id.String() {
		t.Errorf("target %q err %v", target, err)
	}
	if e.th.oks != 1 || e.th.fails != 0 {
		t.Errorf("throttle ok=%d fail=%d", e.th.oks, e.th.fails)
	}
}

func TestLoginAdminHasNullEmail(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "000001", "ADMIN", true)
	rec := e.login(t, creds("000001", password))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if v, ok := body["email"]; !ok || v != nil || body["role"] != "ADMIN" {
		t.Errorf("body = %v: want email null, role ADMIN", body)
	}
	if rows := e.audits(t); len(rows) != 1 || rows[0].Actor != "ADMIN" {
		t.Errorf("audit = %+v", rows)
	}
}

// Each login issues a new ID (no fixation), and the anonymous CSRF token that
// preceded it stops working once the session exists (B1-10 acceptance).
func TestLoginNewSessionIDAndOldCSRFInvalid(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)

	tok, ck := e.csrf(t, nil)
	first := e.loginWith(creds("123456", password), tok, ck)
	s1 := findCookie(first, auth.SessionCookie)
	if first.Code != 200 || s1 == nil {
		t.Fatalf("first login: %d %s", first.Code, first.Body)
	}

	// The same anonymous token against the new session: bound to nothing now.
	stale := e.loginWith(creds("123456", password), tok, ck, s1)
	if stale.Code != 403 || code(t, stale) != "CSRF_INVALID" {
		t.Errorf("old anonymous token with a session: %d %s", stale.Code, stale.Body)
	}

	// A session-bound token works and a second login yields another ID.
	tok2, ck2 := e.csrf(t, s1)
	second := e.loginWith(creds("123456", password), tok2, ck2, s1)
	s2 := findCookie(second, auth.SessionCookie)
	if second.Code != 200 || s2 == nil || s2.Value == s1.Value {
		t.Fatalf("second login: %d, session IDs equal=%v", second.Code, s2 != nil && s2.Value == s1.Value)
	}
	if e.sessions(t) != 2 {
		t.Error("concurrent sessions are allowed: want two rows")
	}
}

// Every refusal is the same 401 with the same body, whatever the reason.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	e.addUser(t, "222222", "USER", false)

	tests := []struct{ name, body, reason string }{
		{"unknown account", creds("999999", password), "unknown_account"},
		{"badly formatted ID", creds("12ab", password), "bad_format"},
		{"empty ID", creds("", password), "bad_format"},
		{"wrong password", creds("123456", "Wrong-Horse-99"), "wrong_password"},
		{"empty password", creds("123456", ""), "wrong_password"},
		{"disabled account, right password", creds("222222", password), "disabled"},
		{"over 72 bytes, even with the right prefix", creds("123456", password+strings.Repeat("x", 70)), "password_too_long"},
	}
	var first string
	for i, tc := range tests {
		rec := e.login(t, tc.body)
		if rec.Code != 401 || code(t, rec) != "INVALID_CREDENTIALS" {
			t.Fatalf("%s: %d %s", tc.name, rec.Code, rec.Body)
		}
		var p map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		delete(p, "requestId")
		b, _ := json.Marshal(p)
		if i == 0 {
			first = string(b)
		} else if string(b) != first {
			t.Errorf("%s: body %s differs from %s", tc.name, b, first)
		}
		if rec.Header().Get("Set-Cookie") != "" {
			t.Errorf("%s: sets a cookie", tc.name)
		}
	}
	if e.sessions(t) != 0 {
		t.Error("a failed login created a session")
	}

	rows := e.audits(t)
	if len(rows) != len(tests) {
		t.Fatalf("audit rows = %d", len(rows))
	}
	for i, tc := range tests {
		r := rows[i]
		if r.Outcome != "FAILED" || r.Actor != "ANONYMOUS" || r.ErrorCode != "INVALID_CREDENTIALS" || r.Status.Int64 != 401 {
			t.Errorf("%s: row %+v", tc.name, r)
		}
		if !strings.Contains(r.After.String, `"`+tc.reason+`"`) {
			t.Errorf("%s: after %q lacks reason %s", tc.name, r.After.String, tc.reason)
		}
	}
	if rows[0].Submitted.String != "999999" {
		t.Errorf("submitted = %q", rows[0].Submitted.String)
	}
	if strings.Contains(rows[3].After.String, password) || strings.Contains(rows[3].After.String, "Wrong-Horse") {
		t.Error("audit row holds a password")
	}
	if e.th.fails != len(tests) {
		t.Errorf("throttle fails = %d, want %d (unknown accounts count too)", e.th.fails, len(tests))
	}
}

func TestLoginMalformedBodyIs400AndAudited(t *testing.T) {
	e := newLoginEnv(t)
	for _, body := range []string{`{"empNo":"123456"}`, `{"empNo":123456,"password":"x"}`, `{`, `{"empNo":null,"password":"x"}`} {
		rec := e.login(t, body)
		if rec.Code != 400 || code(t, rec) != "MALFORMED_REQUEST" {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
	rows := e.audits(t)
	if len(rows) != 4 || rows[0].Outcome != "FAILED" {
		t.Errorf("audit = %+v", rows)
	}
}

func TestLoginThrottled(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	e.th.retry = 90*time.Second + 300*time.Millisecond

	rec := e.login(t, creds("123456", password)) // correct password still throttled
	if rec.Code != 429 || code(t, rec) != "LOGIN_THROTTLED" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Retry-After"); got != "91" {
		t.Errorf("Retry-After = %q, want 91 (rounded up)", got)
	}
	if e.sessions(t) != 0 || e.th.fails != 0 || e.th.oks != 0 {
		t.Errorf("throttled attempt: sessions=%d fails=%d oks=%d", e.sessions(t), e.th.fails, e.th.oks)
	}
	rows := e.audits(t)
	if len(rows) != 1 || rows[0].Outcome != "THROTTLED" || rows[0].Status.Int64 != 429 || rows[0].Submitted.String != "123456" {
		t.Errorf("audit = %+v", rows)
	}
}

func TestLoginThrottleErrorFailsClosed(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	e.th.err = context.DeadlineExceeded
	rec := e.login(t, creds("123456", password))
	if rec.Code != 500 || e.sessions(t) != 0 {
		t.Errorf("status %d sessions %d: want 500 and no session", rec.Code, e.sessions(t))
	}
}

// The session and its audit row commit together: when the audit insert fails,
// no session survives.
func TestLoginAuditFailureRollsBackSession(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	if _, err := e.db.Exec(`CREATE TRIGGER no_login_audit BEFORE INSERT ON audit_log FOR EACH ROW
		SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'audit down'`); err != nil {
		t.Fatal(err)
	}
	rec := e.login(t, creds("123456", password))
	if rec.Code != 500 || e.sessions(t) != 0 {
		t.Errorf("status %d sessions %d: want 500 and no session", rec.Code, e.sessions(t))
	}
}
