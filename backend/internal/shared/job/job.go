// Package job runs background work on a fixed period with random jitter
// (Backend.md §9). Every replica runs its own loop; the work itself must be
// idempotent and safe to run concurrently on several replicas, because no
// leader is elected.
package job

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"
)

// Spec describes one periodic job.
type Spec struct {
	Name     string        // log attribute "job"
	Interval time.Duration // base period between two runs
	Jitter   time.Duration // each wait adds a random [0, Jitter), so replicas drift apart
}

// Run calls fn once per Interval plus jitter until ctx is cancelled, then
// returns. The first call also waits a random [0, Jitter), so replicas started
// together do not run in step. A failing run is logged and the loop goes on;
// a run in progress sees ctx cancelled and should return promptly.
func Run(ctx context.Context, log *slog.Logger, s Spec, fn func(context.Context) error) {
	run(ctx, log, s, fn, rand.Int64N)
}

// run is Run with the random source injected for tests.
func run(ctx context.Context, log *slog.Logger, s Spec, fn func(context.Context) error, randN func(int64) int64) {
	log = log.With("job", s.Name)
	t := time.NewTimer(jitter(s.Jitter, randN))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Debug("job stopped")
			return
		case <-t.C:
		}
		if err := fn(ctx); err != nil && ctx.Err() == nil {
			log.Warn("job run failed", "error", err.Error())
		}
		t.Reset(s.Interval + jitter(s.Jitter, randN))
	}
}

// jitter returns a random duration in [0, upTo), or 0 when upTo <= 0.
func jitter(upTo time.Duration, randN func(int64) int64) time.Duration {
	if upTo <= 0 {
		return 0
	}
	return time.Duration(randN(int64(upTo)))
}
