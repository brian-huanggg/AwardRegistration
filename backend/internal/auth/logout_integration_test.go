//go:build integration

package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

func (e loginEnv) logout(tok string, ck *http.Cookie, extra ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.Header.Set("X-CSRF-Token", tok)
	req.Header.Set("User-Agent", "it/1")
	req.RemoteAddr = "192.0.2.9:4000"
	if ck != nil {
		req.AddCookie(ck)
	}
	for _, c := range extra {
		req.AddCookie(c)
	}
	return e.do(req)
}

func (e loginEnv) logoutAudits(t *testing.T) (n int, actor, empNo string, status int) {
	t.Helper()
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='AUTH_LOGOUT' AND outcome='SUCCESS'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n > 0 {
		if err := e.db.QueryRow(`SELECT actor_role, actor_emp_no, status FROM audit_log WHERE action='AUTH_LOGOUT' LIMIT 1`).
			Scan(&actor, &empNo, &status); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func TestLogoutEndsSession(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	sess := findCookie(e.login(t, creds("123456", password)), auth.SessionCookie)

	tok, ck := e.csrf(t, sess) // token bound to the session
	rec := e.logout(tok, ck, sess)
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("status %d body %q", rec.Code, rec.Body)
	}
	if c := findCookie(rec, auth.SessionCookie); c == nil || c.MaxAge >= 0 || c.Value != "" || !c.Secure || !c.HttpOnly || c.Path != "/" {
		t.Errorf("cookie not cleared: %+v", c)
	}
	if e.sessions(t) != 0 {
		t.Error("session row still present")
	}
	if n, actor, empNo, status := e.logoutAudits(t); n != 1 || actor != "USER" || empNo != "123456" || status != 204 {
		t.Errorf("audit n=%d actor=%s emp=%s status=%d", n, actor, empNo, status)
	}

	// The old cookie no longer authenticates.
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(sess)
	if rec := e.do(req); rec.Code != 401 {
		t.Errorf("after logout: %d, want 401", rec.Code)
	}
	// The session-bound token is useless now: replaying logout is a 403 only
	// because the anonymous binding differs.
	if rec := e.logout(tok, ck, sess); rec.Code != 403 {
		t.Errorf("replay with old token: %d, want 403", rec.Code)
	}
}

func TestLogoutWithoutSessionIsIdempotent(t *testing.T) {
	e := newLoginEnv(t)
	tok, ck := e.csrf(t, nil)
	rec := e.logout(tok, ck)
	if rec.Code != 204 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if n, _, _, _ := e.logoutAudits(t); n != 0 {
		t.Errorf("anonymous logout wrote %d audit rows", n)
	}
	// Expired or junk session cookie behaves the same.
	if rec := e.logout(tok, ck, &http.Cookie{Name: auth.SessionCookie, Value: "junk"}); rec.Code != 204 {
		t.Errorf("junk cookie: %d", rec.Code)
	}
}

func TestLogoutRequiresCSRF(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	sess := findCookie(e.login(t, creds("123456", password)), auth.SessionCookie)

	anonTok, anonCk := e.csrf(t, nil) // wrong binding for this session
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"no token":      e.logout("", nil, sess),
		"anonymous tok": e.logout(anonTok, anonCk, sess),
	} {
		if rec.Code != 403 || code(t, rec) != "CSRF_INVALID" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if e.sessions(t) != 1 {
		t.Error("a rejected logout must not end the session")
	}
	if n, _, _, _ := e.logoutAudits(t); n != 0 {
		t.Error("no SUCCESS row for a rejected logout")
	}
}

func TestLogoutOtherSessionsSurvive(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	a := findCookie(e.login(t, creds("123456", password)), auth.SessionCookie)
	e.login(t, creds("123456", password))
	tok, ck := e.csrf(t, a)
	if rec := e.logout(tok, ck, a); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	if e.sessions(t) != 1 {
		t.Errorf("want 1 remaining session, got %d", e.sessions(t))
	}
}
