// Package server composes the routers and middleware chain.
package server

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

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
// extra (the session middleware, so 401 beats 406) → Accept check.
func NewPublic(log *slog.Logger, mount func(chi.Router), extra ...func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.NotFound(web.NotFound)
	r.MethodNotAllowed(web.MethodNotAllowed)
	if mount != nil {
		mount(r)
	}
	// The chain wraps the router instead of using r.Use: chi skips r.Use
	// middleware (also for NotFound) while no route is registered.
	h := web.AcceptCheck(r, IsFileDownload)
	for i := len(extra) - 1; i >= 0; i-- {
		h = extra[i](h)
	}
	return web.RequestID(web.AccessLog(log)(web.Recover(log)(h)))
}

// SlidesPath is the slide download route under /api/v1.
const SlidesPath = "/api/v1/ideas/{id}/slides"

// IsFileDownload reports whether r targets the slide download, which serves a
// file and therefore skips the JSON Accept check. HEAD is included so that it
// answers 405, never 406. Errors on this route are still problem+json.
func IsFileDownload(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/v1/ideas/")
	if !ok {
		return false
	}
	id, tail, ok := strings.Cut(rest, "/")
	return ok && id != "" && tail == "slides"
}

// Check reports whether one dependency is usable; a non-nil error makes
// readiness answer 503. Only dependencies a request cannot do without belong
// here: the database yes, MinIO no (PRD §8.3, Architecture §12).
type Check func(ctx context.Context) error

// checkTimeout bounds all checks of one readiness request, so a hung database
// cannot make the kubelet probe time out with no answer.
const checkTimeout = 2 * time.Second

// DBCheck pings the database through the pool.
func DBCheck(db *sql.DB) Check { return db.PingContext }

// NewManagement builds the management router: probes only (metrics arrive in a
// later PR). It is never exposed by the ingress.
//
// Liveness answers 200 whenever the process can serve; it checks nothing else,
// so a database outage never restarts pods. Readiness answers 200 only while
// `ready` is true (false before startup completes and after SIGTERM) and every
// check passes.
func NewManagement(log *slog.Logger, ready *atomic.Bool, checks ...Check) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+LivenessPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET "+ReadinessPath, func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
		defer cancel()
		for _, c := range checks {
			if err := c(ctx); err != nil {
				log.Warn("readiness check failed", "error", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}
