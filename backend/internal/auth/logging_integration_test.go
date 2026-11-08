//go:build integration

package auth_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestLogsNeverContainSecrets drives login (good and bad), csrf, me, logout and
// a rejected write at DEBUG level and asserts no secret reaches any log line
// (PRD §6.5 / §8.2: no password, session ID, CSRF token or Cookie in logs).
func TestLogsNeverContainSecrets(t *testing.T) {
	buf := &syncBuf{}
	e := newLoginEnvLog(t, slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	e.addUser(t, "123456", "USER", true)

	const wrongPW = "Wr0ng-Secret-Passw0rd!"
	secrets := []string{password, wrongPW}

	e.login(t, creds("123456", wrongPW))
	e.login(t, creds("999999", wrongPW))
	e.login(t, `{"empNo":"123456","password":"`+wrongPW+`"`) // malformed JSON echoing the password

	rec := e.login(t, creds("123456", password))
	sess := findCookie(rec, auth.SessionCookie)
	if sess == nil {
		t.Fatalf("login failed: %d", rec.Code)
	}
	secrets = append(secrets, sess.Value)

	tok, ck := e.csrf(t, sess)
	secrets = append(secrets, tok, ck.Value)

	if rec := e.me(t, true, sess.Value); rec.Code != 200 {
		t.Fatalf("me: %d", rec.Code)
	}
	// Write with a bad CSRF token: denied, and the submitted token must not be logged.
	bad := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	bad.Header.Set("X-CSRF-Token", "forged-token-value-123")
	bad.AddCookie(sess)
	bad.AddCookie(&http.Cookie{Name: "__Host-csrf", Value: "forged-cookie-value-456"})
	e.do(bad)
	secrets = append(secrets, "forged-token-value-123", "forged-cookie-value-456")

	if rec := e.logout(tok, ck, sess); rec.Code != 204 {
		t.Fatalf("logout: %d", rec.Code)
	}

	out := buf.String()
	if strings.TrimSpace(out) == "" {
		t.Fatal("no log output captured; the test would prove nothing")
	}
	for _, s := range secrets {
		if s != "" && strings.Contains(out, s) {
			t.Errorf("log contains secret %q", s)
		}
	}
	for _, name := range []string{"Set-Cookie", "set-cookie", "__Host-session=", "X-CSRF-Token"} {
		if strings.Contains(out, name) {
			t.Errorf("log contains header material %q", name)
		}
	}
}
