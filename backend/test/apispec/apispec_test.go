package apispec_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/test/apispec"
)

// fakeT records failures instead of failing the real test.
type fakeT struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (f *fakeT) Helper() {}
func (f *fakeT) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, format)
}

func serve(t *testing.T, status int, ctype, body, path string) *fakeT {
	t.Helper()
	spec := apispec.Load(t)
	ft := &fakeT{TB: t}
	h := spec.Validate(ft, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, http.NoBody))
	if rec.Code != status || rec.Body.String() != body {
		t.Fatalf("response not passed through: %d %q", rec.Code, rec.Body)
	}
	return ft
}

func TestConformingResponsePasses(t *testing.T) {
	ft := serve(t, 200, "application/json",
		`{"deadline":null,"state":"NOT_OPEN","serverTime":"2026-01-01T00:00:00.000Z"}`, "/api/v1/deadline")
	if len(ft.errs) != 0 {
		t.Fatalf("unexpected failure: %v", ft.errs)
	}
	ft = serve(t, 503, "application/problem+json",
		`{"type":"about:blank","title":"x","status":503,"code":"SERVICE_UNAVAILABLE"}`, "/api/v1/deadline")
	if len(ft.errs) != 0 {
		t.Fatalf("problem response rejected: %v", ft.errs)
	}
}

func TestWrongResponseFails(t *testing.T) {
	for name, tc := range map[string]struct {
		status      int
		ctype, body string
		path        string
	}{
		"wrong field type":      {200, "application/json", `{"deadline":null,"state":"NOT_OPEN","serverTime":5}`, "/api/v1/deadline"},
		"missing field":         {200, "application/json", `{"deadline":null,"state":"NOT_OPEN"}`, "/api/v1/deadline"},
		"state not in enum":     {200, "application/json", `{"deadline":null,"state":"WRONG","serverTime":"2026-01-01T00:00:00.000Z"}`, "/api/v1/deadline"},
		"undeclared status":     {418, "application/json", `{}`, "/api/v1/deadline"},
		"problem without code":  {503, "application/problem+json", `{"type":"about:blank","title":"x","status":503}`, "/api/v1/deadline"},
		"operation not in spec": {200, "application/json", `{}`, "/api/v1/nope"},
	} {
		t.Run(name, func(t *testing.T) {
			if ft := serve(t, tc.status, tc.ctype, tc.body, tc.path); len(ft.errs) == 0 {
				t.Fatal("mismatch was not reported")
			}
		})
	}
}
