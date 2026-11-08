//go:build integration

package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

func (e loginEnv) me(t *testing.T, withSession bool, sid string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	if withSession {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	}
	return e.do(req)
}

func TestMeReturnsCaller(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	e.addUser(t, "900001", "ADMIN", true)

	for _, tc := range []struct {
		emp, role string
		email     any
	}{{"123456", "USER", "123456@example.com"}, {"900001", "ADMIN", nil}} {
		sess := findCookie(e.login(t, creds(tc.emp, password)), auth.SessionCookie)
		rec := e.me(t, true, sess.Value)
		if rec.Code != 200 {
			t.Fatalf("%s: status %d", tc.emp, rec.Code)
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got["empNo"] != tc.emp || got["role"] != tc.role || got["email"] != tc.email || got["name"] != "N "+tc.emp {
			t.Errorf("%s: body %v", tc.emp, got)
		}
		if _, leaked := got["passwordHash"]; leaked || len(got) != 4 {
			t.Errorf("%s: unexpected fields %v", tc.emp, got)
		}
	}
}

func TestMeWithoutValidSessionIs401(t *testing.T) {
	e := newLoginEnv(t)
	if rec := e.me(t, false, ""); rec.Code != 401 || code(t, rec) != "UNAUTHENTICATED" {
		t.Errorf("no cookie: %d", rec.Code)
	}
	if rec := e.me(t, true, "not-a-session"); rec.Code != 401 {
		t.Errorf("bad cookie: %d", rec.Code)
	}
}

func TestMeAfterAccountDisabledIs401(t *testing.T) {
	e := newLoginEnv(t)
	id := e.addUser(t, "123456", "USER", true)
	sess := findCookie(e.login(t, creds("123456", password)), auth.SessionCookie)
	if _, err := e.db.Exec(`UPDATE app_user SET enabled = 0 WHERE id = ?`, id.String()); err != nil {
		t.Fatal(err)
	}
	if rec := e.me(t, true, sess.Value); rec.Code != 401 {
		t.Errorf("disabled: %d, want 401", rec.Code)
	}
}
