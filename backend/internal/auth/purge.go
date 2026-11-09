package auth

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
)

// Purge schedule (B1b-07). Expired rows already fail every check, so the purge
// only bounds table size; a few minutes of delay change nothing.
const (
	PurgeInterval = 5 * time.Minute
	PurgeJitter   = time.Minute
	purgeBatch    = 1000
)

// Purger deletes dead rows of the session and throttle tables: expired
// sessions, login_lock rows without an active lock whose last failure left the
// window, login_attempt rows outside the IP window and expired
// login_throttle_audit claims. Each DELETE removes at most Batch rows, so row
// locks are held briefly, and repeats until a short batch.
//
// Every replica runs its own Purger and no leader is elected. That is safe:
// each condition only matches rows that every reader already ignores, a
// DELETE of a row that is already gone affects nothing, and two runners on the
// same rows only wait for each other's row locks. A deadlock or lock-wait
// timeout ends that table's turn without an error; the next run continues.
type Purger struct {
	DB    *sql.DB
	Log   *slog.Logger
	Batch int // rows per DELETE; 0 means 1000
}

// Purge runs one pass over all four tables. The returned error is the first
// non-contention database error; the remaining tables are still purged.
func (p Purger) Purge(ctx context.Context) error {
	batch := p.Batch
	if batch <= 0 {
		batch = purgeBatch
	}
	tables := []struct {
		name string
		del  func(context.Context, *sql.DB, int) (int64, error)
	}{
		{"session", func(c context.Context, d *sql.DB, n int) (int64, error) { return store.DeleteExpired(c, d, limits, n) }},
		{"login_lock", func(c context.Context, d *sql.DB, n int) (int64, error) {
			return store.DeleteStaleLocks(c, d, lockPolicy, n)
		}},
		{"login_attempt", func(c context.Context, d *sql.DB, n int) (int64, error) {
			return store.DeleteOldAttempts(c, d, ipPolicy, n)
		}},
		{"login_throttle_audit", store.DeleteExpiredThrottleAudits},
	}
	var first error
	for _, t := range tables {
		n, err := drain(ctx, p.DB, batch, t.del)
		if ctx.Err() != nil {
			return ctx.Err() // shutdown; not a failure
		}
		switch {
		case err != nil && db.IsContention(err):
			p.Log.Info("purge yielded to another runner", "table", t.name, "deleted", n)
		case err != nil:
			p.Log.Warn("purge failed", "table", t.name, "deleted", n, "error", err.Error())
			if first == nil {
				first = err
			}
		case n > 0:
			p.Log.Info("purged", "table", t.name, "deleted", n)
		}
	}
	return first
}

// drain repeats del until it deletes fewer than batch rows and returns the
// total.
func drain(ctx context.Context, d *sql.DB, batch int, del func(context.Context, *sql.DB, int) (int64, error)) (int64, error) {
	var total int64
	for {
		n, err := del(ctx, d, batch)
		total += n
		if err != nil || n < int64(batch) {
			return total, err
		}
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
}
