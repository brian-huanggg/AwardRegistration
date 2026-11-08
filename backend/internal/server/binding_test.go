package server_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
)

// stub implements only what the binding tests reach; everything else is 501.
type stub struct{ api.Unimplemented }

func (stub) GetIdea(w http.ResponseWriter, _ *http.Request, _ api.IdeaId) {
	w.WriteHeader(http.StatusOK)
}
func (stub) ListIdeas(w http.ResponseWriter, _ *http.Request, _ api.ListIdeasParams) {
	w.WriteHeader(http.StatusOK)
}
func (stub) Logout(w http.ResponseWriter, _ *http.Request, _ api.LogoutParams) {
	w.WriteHeader(http.StatusNoContent)
}

func TestBindingErrors(t *testing.T) {
	h := server.NewPublic(slog.New(slog.DiscardHandler), server.Mount(stub{}))
	tests := []struct {
		name, method, path string
		status             int
		code, field, fcode string
	}{
		{"bad UUID in path is the uniform 404", "GET", "/api/v1/ideas/not-a-uuid", 404, "NOT_FOUND", "", ""},
		{"valid UUID binds", "GET", "/api/v1/ideas/6f1c2c1e-0000-4000-8000-000000000000", 200, "", "", ""},
		{"page=abc is 422", "GET", "/api/v1/ideas?page=abc", 422, "VALIDATION_FAILED", "page", "INVALID_FORMAT"},
		{"size=1.5 is 422", "GET", "/api/v1/ideas?size=1.5", 422, "VALIDATION_FAILED", "size", "INVALID_FORMAT"},
		{"repeated page is 422", "GET", "/api/v1/ideas?page=1&page=2", 422, "VALIDATION_FAILED", "page", "INVALID_FORMAT"},
		{"valid query binds", "GET", "/api/v1/ideas?page=2&size=10", 200, "", "", ""},
		{"missing CSRF header is not a binding error", "POST", "/api/v1/logout-not-routed", 404, "NOT_FOUND", "", ""},
		{"logout without header reaches the handler", "POST", "/api/v1/auth/logout", 204, "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, http.NoBody))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body)
			}
			if tc.code == "" {
				return
			}
			var p struct {
				Code   string `json:"code"`
				Errors []struct{ Field, Code string }
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Code != tc.code {
				t.Fatalf("code = %q, want %q", p.Code, tc.code)
			}
			if tc.field != "" && (len(p.Errors) != 1 || p.Errors[0].Field != tc.field || p.Errors[0].Code != tc.fcode) {
				t.Fatalf("errors = %+v, want %s/%s", p.Errors, tc.field, tc.fcode)
			}
		})
	}
}

func TestBindingErrorOtherIsMalformed(t *testing.T) {
	rec := httptest.NewRecorder()
	server.BindingError(rec, httptest.NewRequest("GET", "/", http.NoBody), http.ErrNotSupported)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
