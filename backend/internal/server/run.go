package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
)

// Run starts the public and management servers and blocks until ctx is
// cancelled (SIGTERM) or a server fails. Shutdown order: readiness false, drain
// delay, http.Server.Shutdown with a timeout, return.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger, public http.Handler, ready *atomic.Bool) error {
	pub := newHTTPServer(cfg.PublicAddr, public)
	mgmt := newHTTPServer(cfg.ManagementAddr, NewManagement(ready))

	errc := make(chan error, 2)
	for name, s := range map[string]*http.Server{"public": pub, "management": mgmt} {
		go func() {
			log.Info("listening", "server", name, "addr", s.Addr)
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- fmt.Errorf("%s server: %w", name, err)
			}
		}()
	}
	ready.Store(true)

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errc:
	}

	ready.Store(false)
	if cfg.ReadyDrainDelay > 0 {
		time.Sleep(cfg.ReadyDrainDelay)
	}
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	log.Info("shutting down")
	return errors.Join(runErr, pub.Shutdown(sctx), mgmt.Shutdown(sctx))
}

func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
