package auth_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

const (
	keyA = "0123456789abcdef0123456789abcdef"
	keyB = "fedcba9876543210fedcba9876543210"
)

func sid(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func mustCSRF(t *testing.T, cur string, prev ...string) *auth.CSRF {
	t.Helper()
	c, err := auth.NewCSRF(cur, prev...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type denial struct{ action, reason string }

// csrfChain mirrors production order: session middleware, then CSRF, then router.
func csrfChain(c *auth.CSRF, v auth.Validator, got *denial) http.Handler {
	r := chi.NewRouter()
	r.NotFound(web.NotFound)
	r.MethodNotAllowed(web.MethodNotAllowed)
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	r.Get("/auth/csrf", auth.Handler{CSRF: c, Log: slog.New(slog.DiscardHandler)}.GetCsrfToken)
	r.Post("/w", ok)
	r.Put("/w", ok)
	r.Delete("/w", ok)
	r.Post("/auth/logout", ok)
	fail := func(w http.ResponseWriter, req *http.Request, err error) {
		var e *web.Error
		if errors.As(err, &e) && got != nil {
			*got = denial{e.AuditAction, e.AuditReason}
		}
		web.WriteError(w, req, err)
	}
	action := func(req *http.Request) string {
		if req.URL.Path == "/auth/logout" {
			return "AUTH_LOGOUT"
		}
		return ""
	}
	public := func(req *http.Request) bool { return req.URL.Path == "/auth/csrf" || req.URL.Path == "/auth/logout" }
	log := slog.New(slog.DiscardHandler)
	return web.RequestID(auth.Middleware(v, public, log)(c.Middleware(fail, action)(r)))
}

func fetch(t *testing.T, h http.Handler, session string) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/auth/csrf", nil)
	if session != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: session})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /auth/csrf = %d %s", rec.Code, rec.Body)
	}
	var body struct{ Token string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	var ck *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CSRFCookie {
			ck = c
		}
	}
	if ck == nil || ck.Value != body.Token {
		t.Fatalf("cookie %v does not match body token", ck)
	}
	if !ck.HttpOnly || !ck.Secure || ck.Path != "/" || ck.Domain != "" || ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie attributes: %+v", ck)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	return body.Token
}

func write(method, path, session, cookie, header string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader("ignored"))
	if session != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: session})
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.CSRFCookie, Value: cookie})
	}
	if header != "" {
		req.Header.Set(auth.CSRFHeader, header)
	}
	return req
}

func TestCSRFWrites(t *testing.T) {
	c := mustCSRF(t, keyA)
	s1, s2 := sid(t), sid(t)
	user := security.Principal{UserID: uuid.New(), EmpNo: "U1", Role: security.RoleUser}
	v := &fakeValidator{live: map[string]security.Principal{s1: user, s2: user}}
	h := csrfChain(c, v, nil)

	anon, tok1, tok2 := fetch(t, h, ""), fetch(t, h, s1), fetch(t, h, s2)
	other := mustCSRF(t, keyB)
	foreign, _ := other.Issue([]byte("x"))

	tests := []struct {
		name                    string
		method, path            string
		session, cookie, header string
		status                  int
	}{
		{"session token, own session", "POST", "/w", s1, tok1, tok1, 204},
		{"anonymous token, no session", "POST", "/auth/logout", "", anon, anon, 204},
		{"PUT and DELETE are checked", "PUT", "/w", s1, "", "", 403},
		{"DELETE with token", "DELETE", "/w", s1, tok1, tok1, 204},
		{"no header", "POST", "/w", s1, tok1, "", 403},
		{"no cookie", "POST", "/w", s1, "", tok1, 403},
		{"cookie and header differ", "POST", "/w", s1, tok1, tok2, 403},
		{"other session's token", "POST", "/w", s1, tok2, tok2, 403},
		{"anonymous token against a session", "POST", "/w", s1, anon, anon, 403},
		{"session token without a session (logout)", "POST", "/auth/logout", "", tok1, tok1, 403},
		{"session token, stale session cookie (logout)", "POST", "/auth/logout", sid(t), tok1, tok1, 403},
		{"signed by an unknown key", "POST", "/w", s1, foreign, foreign, 403},
		{"garbage", "POST", "/w", s1, "abc", "abc", 403},
		{"truncated signature", "POST", "/w", s1, tok1[:len(tok1)-2], tok1[:len(tok1)-2], 403},
		{"no dot", "POST", "/w", s1, strings.ReplaceAll(tok1, ".", ""), strings.ReplaceAll(tok1, ".", ""), 403},
		{"403 beats 404 on unknown path", "POST", "/nope", s1, "", "", 403},
		{"unknown path, valid token, is 404", "POST", "/nope", s1, tok1, tok1, 404},
		{"401 beats 403", "POST", "/w", "", "", "", 401},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, write(tc.method, tc.path, tc.session, tc.cookie, tc.header))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body)
			}
			if tc.status == 403 && !strings.Contains(rec.Body.String(), `"code":"CSRF_INVALID"`) {
				t.Errorf("body %s", rec.Body)
			}
		})
	}
}

func TestCSRFSafeMethodsAreNotChecked(t *testing.T) {
	h := csrfChain(mustCSRF(t, keyA), &fakeValidator{}, nil)
	for _, m := range []string{"GET", "HEAD", "OPTIONS"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(m, "/auth/csrf", nil))
		if rec.Code == 403 {
			t.Errorf("%s was CSRF-checked", m)
		}
	}
}

func TestCSRFNeverReadsBody(t *testing.T) {
	h := csrfChain(mustCSRF(t, keyA), &fakeValidator{}, nil)
	req := write("POST", "/auth/logout", "", "", "")
	req.Body = failReader{t}
	h.ServeHTTP(httptest.NewRecorder(), req)
}

type failReader struct{ t *testing.T }

func (f failReader) Read([]byte) (int, error) { f.t.Error("body was read"); return 0, context.Canceled }
func (failReader) Close() error               { return nil }

func TestCSRFDenialAuditFields(t *testing.T) {
	c := mustCSRF(t, keyA)
	anon := fetch(t, csrfChain(c, &fakeValidator{}, nil), "")
	for _, tc := range []struct{ name, cookie, header, reason string }{
		{"missing", "", "", "csrf_missing"},
		{"mismatch", anon, anon + "x", "csrf_mismatch"},
		{"bad signature", "a.b", "a.b", "csrf_invalid_binding"},
	} {
		var d denial
		rec := httptest.NewRecorder()
		csrfChain(c, &fakeValidator{}, &d).ServeHTTP(rec, write("POST", "/auth/logout", "", tc.cookie, tc.header))
		if rec.Code != 403 || d != (denial{"AUTH_LOGOUT", tc.reason}) {
			t.Errorf("%s: status %d, audit %+v", tc.name, rec.Code, d)
		}
	}
	// Unrouted write: denied, but nothing to audit.
	var d denial
	csrfChain(c, &fakeValidator{live: map[string]security.Principal{"x": {}}}, &d).
		ServeHTTP(httptest.NewRecorder(), write("POST", "/w", "", "", ""))
	if d != (denial{}) {
		t.Errorf("unknown action was audited: %+v", d)
	}
}

func TestCSRFTokensAreFresh(t *testing.T) {
	h := csrfChain(mustCSRF(t, keyA), &fakeValidator{}, nil)
	first, second := fetch(t, h, ""), fetch(t, h, "")
	if first == second {
		t.Error("two tokens are equal")
	}
}

func TestCSRFKeyRotation(t *testing.T) {
	b := []byte("binding")
	old := mustCSRF(t, keyA)
	tok, _ := old.Issue(b)

	rotated := mustCSRF(t, keyB, keyA)
	if !rotated.Verify(tok, b) {
		t.Error("previous key must still verify")
	}
	if fresh, _ := rotated.Issue(b); !mustCSRF(t, keyB).Verify(fresh, b) || old.Verify(fresh, b) {
		t.Error("new tokens must be signed by the current key only")
	}
	if mustCSRF(t, keyB).Verify(tok, b) {
		t.Error("after the previous key is dropped the old token must fail")
	}
	if !mustCSRF(t, keyB, "").Verify(mustTok(t, mustCSRF(t, keyB), b), b) {
		t.Error("empty previous key means none")
	}
	if old.Verify(tok, []byte("other")) {
		t.Error("binding must be part of the signature")
	}
}

func mustTok(t *testing.T, c *auth.CSRF, b []byte) string {
	t.Helper()
	tok, err := c.Issue(b)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestNewCSRFRejectsShortKeys(t *testing.T) {
	if _, err := auth.NewCSRF("short"); err == nil {
		t.Error("short current key accepted")
	}
	if _, err := auth.NewCSRF(keyA, "short"); err == nil {
		t.Error("short previous key accepted")
	}
}

func TestCSRFSessionWithUnbindableIDIs500(t *testing.T) {
	// A validator that accepts an ID Binding cannot parse is a wiring bug: fail closed.
	v := &fakeValidator{live: map[string]security.Principal{"not-an-id": {Role: security.RoleUser}}}
	h := csrfChain(mustCSRF(t, keyA), v, nil)
	req := httptest.NewRequest("GET", "/auth/csrf", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "not-an-id"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 500 {
		t.Errorf("status = %d", rec.Code)
	}
}
