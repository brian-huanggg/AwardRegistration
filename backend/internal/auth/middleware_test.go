package auth_test

import (
	"context"
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

type fakeValidator struct {
	live  map[string]security.Principal
	err   error
	calls int
}

func (f *fakeValidator) Validate(_ context.Context, id string) (security.Principal, bool, error) {
	f.calls++
	if f.err != nil {
		return security.Principal{}, false, f.err
	}
	p, ok := f.live[id]
	return p, ok, nil
}

func isPublic(r *http.Request) bool {
	return r.Method == "GET" && r.URL.Path == "/open"
}

// chain mirrors production: middleware wraps the router, whose NotFound and
// MethodNotAllowed hooks are the web ones.
func chain(v auth.Validator, seen *security.Principal) http.Handler {
	r := chi.NewRouter()
	r.NotFound(web.NotFound)
	r.MethodNotAllowed(web.MethodNotAllowed)
	h := func(w http.ResponseWriter, req *http.Request) {
		if p, ok := security.PrincipalFrom(req.Context()); ok && seen != nil {
			*seen = p
		}
		w.WriteHeader(http.StatusNoContent)
	}
	r.Get("/open", h)
	r.Get("/closed", h)
	return web.RequestID(auth.Middleware(v, isPublic, slog.New(slog.DiscardHandler))(r))
}

func request(method, path, cookie string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: cookie})
	}
	return req
}

func TestMiddlewareCombinations(t *testing.T) {
	user := security.Principal{UserID: uuid.New(), EmpNo: "U1", Role: security.RoleUser}
	v := &fakeValidator{live: map[string]security.Principal{"good": user}}

	tests := []struct {
		name         string
		method, path string
		cookie       string
		status       int
		code         string
		principal    bool
	}{
		{"public, no session", "GET", "/open", "", 204, "", false},
		{"public, valid session", "GET", "/open", "good", 204, "", true},
		{"public, invalid session is anonymous", "GET", "/open", "stale", 204, "", false},
		{"public path, other method, no session", "POST", "/open", "", 401, "UNAUTHENTICATED", false},
		{"public path, other method, session", "POST", "/open", "good", 405, "METHOD_NOT_ALLOWED", false},
		{"protected, no session", "GET", "/closed", "", 401, "UNAUTHENTICATED", false},
		{"protected, invalid session", "GET", "/closed", "stale", 401, "UNAUTHENTICATED", false},
		{"protected, valid session", "GET", "/closed", "good", 204, "", true},
		{"unknown path, no session", "GET", "/nope", "", 401, "UNAUTHENTICATED", false},
		{"unknown path, invalid session", "GET", "/nope", "stale", 401, "UNAUTHENTICATED", false},
		{"unknown path, session", "GET", "/nope", "good", 404, "NOT_FOUND", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var seen security.Principal
			rec := httptest.NewRecorder()
			chain(v, &seen).ServeHTTP(rec, request(tc.method, tc.path, tc.cookie))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body)
			}
			if tc.code != "" && !strings.Contains(rec.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("body %s lacks code %s", rec.Body, tc.code)
			}
			if tc.code != "" && rec.Header().Get("Content-Type") != web.ProblemContentType {
				t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
			}
			if got := seen == user; got != tc.principal {
				t.Errorf("principal in context = %v, want %v", got, tc.principal)
			}
		})
	}
}

func TestMiddlewareNoCookieSkipsStore(t *testing.T) {
	v := &fakeValidator{}
	chain(v, nil).ServeHTTP(httptest.NewRecorder(), request("GET", "/open", ""))
	// An empty cookie value is the same as none.
	req := httptest.NewRequest("GET", "/open", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: ""})
	chain(v, nil).ServeHTTP(httptest.NewRecorder(), req)
	if v.calls != 0 {
		t.Errorf("store called %d times without a cookie value", v.calls)
	}
}

func TestMiddlewareIgnoresOtherCookies(t *testing.T) {
	v := &fakeValidator{live: map[string]security.Principal{"good": {Role: security.RoleUser}}}
	req := httptest.NewRequest("GET", "/closed", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "good"})
	rec := httptest.NewRecorder()
	chain(v, nil).ServeHTTP(rec, req)
	if rec.Code != 401 || v.calls != 0 {
		t.Errorf("status %d, store calls %d", rec.Code, v.calls)
	}
}

func TestMiddlewareStoreFailureIs500NotPublicFallthrough(t *testing.T) {
	v := &fakeValidator{err: errors.New("db down")}
	for _, path := range []string{"/open", "/closed", "/nope"} {
		rec := httptest.NewRecorder()
		chain(v, nil).ServeHTTP(rec, request("GET", path, "good"))
		if rec.Code != 500 || !strings.Contains(rec.Body.String(), "INTERNAL_ERROR") {
			t.Errorf("%s: status %d body %s", path, rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "db down") {
			t.Errorf("%s: cause leaked: %s", path, rec.Body)
		}
	}
}

func TestMiddlewareNilPublicMeansAllProtected(t *testing.T) {
	h := auth.Middleware(&fakeValidator{}, nil, slog.New(slog.DiscardHandler))(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("GET", "/open", ""))
	if rec.Code != 401 {
		t.Errorf("status = %d", rec.Code)
	}
}
