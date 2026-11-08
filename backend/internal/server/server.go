// Package server composes the routers and middleware chain.
package server

import (
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/go-chi/chi/v5"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Probe paths are ADR-0001 O3 (open); /actuator paths are kept until decided.
const (
	LivenessPath  = "/actuator/health/liveness"
	ReadinessPath = "/actuator/health/readiness"
)

// NewPublic builds the public router. Module routes are mounted by `mount`
// (nil in the skeleton) under the chain: request ID → access log → recover →
// Accept check.
func NewPublic(log *slog.Logger, mount func(chi.Router)) http.Handler {
	r := chi.NewRouter()
	r.NotFound(web.NotFound)
	r.MethodNotAllowed(web.MethodNotAllowed)
	if mount != nil {
		mount(r)
	}
	// The chain wraps the router instead of using r.Use: chi skips r.Use
	// middleware (also for NotFound) while no route is registered.
	return web.RequestID(web.AccessLog(log)(web.Recover(log)(web.AcceptCheck(r))))
}

// NewManagement builds the management router: probes only (metrics and the DB
// readiness check arrive in later PRs). It is never exposed by the ingress.
func NewManagement(ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+LivenessPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET "+ReadinessPath, func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}
