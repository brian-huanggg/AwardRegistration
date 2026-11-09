package server_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// downloadStub counts DownloadSlides calls; every other operation is 501.
type downloadStub struct {
	api.Unimplemented
	calls *int
}

func (s downloadStub) DownloadSlides(w http.ResponseWriter, _ *http.Request, _ api.IdeaId) {
	*s.calls++
	w.Header().Set("Content-Type", "application/pdf")
	_, _ = w.Write([]byte("%PDF-"))
}

func TestSlidesDownloadRoute(t *testing.T) {
	const path = "/api/v1/ideas/7f0c1a52-2a51-4f0e-9a43-0d6b5f0b2a11/slides"
	for _, tc := range []struct {
		name, method, path, accept string
		status                     int
		code                       string
		calls                      int
	}{
		// D47/D59: HEAD answers 405 and never runs the GET handler (which audits).
		{"HEAD is 405", http.MethodHead, path, "", 405, "METHOD_NOT_ALLOWED", 0},
		{"HEAD with a file Accept is still 405, not 406", http.MethodHead, path, "application/pdf", 405, "METHOD_NOT_ALLOWED", 0},
		{"GET with a file Accept is served", http.MethodGet, path, "application/pdf", 200, "", 1},
		{"GET with a browser Accept is served", http.MethodGet, path, "text/html,application/xhtml+xml", 200, "", 1},
		{"GET with no Accept is served", http.MethodGet, path, "", 200, "", 1},
		{"malformed ID is the uniform 404", http.MethodGet, "/api/v1/ideas/nope/slides", "", 404, "NOT_FOUND", 0},
		{"other routes keep the 406", http.MethodGet, "/api/v1/ideas/7f0c1a52-2a51-4f0e-9a43-0d6b5f0b2a11", "application/pdf", 406, "NOT_ACCEPTABLE", 0},
		{"wrong method on the download is 405", http.MethodPost, path, "", 405, "METHOD_NOT_ALLOWED", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := server.NewPublic(slog.New(slog.DiscardHandler), server.Mount(downloadStub{calls: &calls}))
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status || calls != tc.calls {
				t.Fatalf("status %d calls %d, want %d %d (%s)", rec.Code, calls, tc.status, tc.calls, rec.Body)
			}
			if tc.code != "" {
				if ct := rec.Header().Get("Content-Type"); ct != web.ProblemContentType {
					t.Errorf("Content-Type %q", ct)
				}
			}
		})
	}
}

func TestIsFileDownload(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{http.MethodGet, "/api/v1/ideas/x/slides", true},
		{http.MethodHead, "/api/v1/ideas/x/slides", true},
		{http.MethodPut, "/api/v1/ideas/x/slides", false},
		{http.MethodGet, "/api/v1/ideas//slides", false},
		{http.MethodGet, "/api/v1/ideas/x/slides/", false},
		{http.MethodGet, "/api/v1/ideas/x", false},
		{http.MethodGet, "/api/v1/ideas/x/members/slides", false},
	} {
		if got := server.IsFileDownload(httptest.NewRequest(tc.method, tc.path, nil)); got != tc.want {
			t.Errorf("%s %s = %v", tc.method, tc.path, got)
		}
	}
}
