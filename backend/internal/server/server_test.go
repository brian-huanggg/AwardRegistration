package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func public() http.Handler {
	return server.NewPublic(slog.New(slog.DiscardHandler), func(r chi.Router) {
		r.Get("/api/v1/thing", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) })
		r.Group(func(r chi.Router) {
			r.Use(web.RequireJSON, web.LimitBody(16<<10, web.CodeRequestTooLarge))
			r.Post("/api/v1/json", func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					if web.IsBodyTooLarge(err) {
						web.WriteError(w, r, web.NewError(web.CodeRequestTooLarge, ""))
						return
					}
					web.WriteError(w, r, err)
				}
			})
		})
	})
}

func do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	public().ServeHTTP(rec, req)
	return rec
}

func TestTransportErrors(t *testing.T) {
	big := strings.Repeat("a", 16<<10+1)
	tests := []struct {
		name   string
		req    *http.Request
		status int
		code   string
	}{
		{"404 unknown path", httptest.NewRequest("GET", "/nope", nil), 404, "NOT_FOUND"},
		{"405 wrong method", httptest.NewRequest("DELETE", "/api/v1/thing", nil), 405, "METHOD_NOT_ALLOWED"},
		{"405 HEAD on GET route", httptest.NewRequest("HEAD", "/api/v1/thing", nil), 405, "METHOD_NOT_ALLOWED"},
		{"406 accept", func() *http.Request {
			r := httptest.NewRequest("GET", "/api/v1/thing", nil)
			r.Header.Set("Accept", "text/html")
			return r
		}(), 406, "NOT_ACCEPTABLE"},
		{"415 content type", func() *http.Request {
			r := httptest.NewRequest("POST", "/api/v1/json", strings.NewReader("x"))
			r.Header.Set("Content-Type", "text/plain")
			return r
		}(), 415, "UNSUPPORTED_MEDIA_TYPE"},
		{"413 declared length", func() *http.Request {
			r := httptest.NewRequest("POST", "/api/v1/json", strings.NewReader(big))
			r.Header.Set("Content-Type", "application/json")
			return r
		}(), 413, "REQUEST_TOO_LARGE"},
		{"413 streamed", func() *http.Request {
			r := httptest.NewRequest("POST", "/api/v1/json", io.NopCloser(strings.NewReader(big)))
			r.ContentLength = -1
			r.Header.Set("Content-Type", "application/json")
			return r
		}(), 413, "REQUEST_TOO_LARGE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(tc.req)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d", rec.Code, tc.status)
			}
			if rec.Header().Get("Content-Type") != web.ProblemContentType {
				t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
			}
			var p struct{ Code, RequestID string }
			_ = json.Unmarshal(rec.Body.Bytes(), &p)
			if p.Code != tc.code || p.RequestID == "" {
				t.Fatalf("body %s", rec.Body)
			}
		})
	}
}

func TestAcceptAccepted(t *testing.T) {
	for _, a := range []string{"", "*/*", "application/json", "text/html, application/json;q=0.9", "application/problem+json"} {
		r := httptest.NewRequest("GET", "/api/v1/thing", nil)
		if a != "" {
			r.Header.Set("Accept", a)
		}
		if rec := do(r); rec.Code != 200 {
			t.Errorf("Accept %q → %d", a, rec.Code)
		}
	}
}

func TestProbes(t *testing.T) {
	var ready atomic.Bool
	h := server.NewManagement(&ready)
	get := func(p string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		return rec.Code
	}
	if get(server.LivenessPath) != 200 || get(server.ReadinessPath) != 503 {
		t.Fatal("before ready")
	}
	ready.Store(true)
	if get(server.ReadinessPath) != 200 {
		t.Fatal("after ready")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestRunGracefulShutdown(t *testing.T) {
	cfg := config.Config{PublicAddr: freeAddr(t), ManagementAddr: freeAddr(t), ShutdownTimeout: 5 * time.Second}
	var ready atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx, cfg, slog.New(slog.DiscardHandler), public(), &ready) }()

	url := "http://" + cfg.ManagementAddr + server.ReadinessPath
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("server never became ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	resp, err := http.Get("http://" + cfg.PublicAddr + "/api/v1/thing")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("public: %v %v", resp, err)
	}
	_ = resp.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no shutdown")
	}
	if ready.Load() {
		t.Fatal("readiness must be false after shutdown")
	}
}

func TestRunFailsWhenPortTaken(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	cfg := config.Config{PublicAddr: l.Addr().String(), ManagementAddr: freeAddr(t), ShutdownTimeout: time.Second}
	var ready atomic.Bool
	if err := server.Run(context.Background(), cfg, slog.New(slog.DiscardHandler), public(), &ready); err == nil {
		t.Fatal("want listen error")
	}
}

func TestChainAppliesWithoutRoutes(t *testing.T) {
	rec := httptest.NewRecorder()
	server.NewPublic(slog.New(slog.DiscardHandler), nil).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 404 || rec.Header().Get(web.RequestIDHeader) == "" || !strings.Contains(rec.Body.String(), `"requestId"`) {
		t.Fatalf("%d %v %s", rec.Code, rec.Header(), rec.Body)
	}
}
