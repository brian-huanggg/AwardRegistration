package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ClaimThrottleAudit reports whether the caller may write the THROTTLED audit
// row of (scope, subject): true for the first throttled attempt of a window,
// false while an earlier claim is still running. A winning claim holds until
// database now + hold. The claim is atomic, so concurrent attempts on several
// replicas produce exactly one winner.
func ClaimThrottleAudit(ctx context.Context, db *sql.DB, scope, subject string, hold time.Duration) (bool, error) {
	us := hold.Microseconds()
	res, err := db.ExecContext(ctx,
		`UPDATE login_throttle_audit SET audited_until = NOW(6) + INTERVAL ? MICROSECOND
		  WHERE scope = ? AND subject = ? AND audited_until <= NOW(6)`, us, scope, subject)
	if err != nil {
		return false, fmt.Errorf("claim throttle audit: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}
	res, err = db.ExecContext(ctx,
		`INSERT IGNORE INTO login_throttle_audit (scope, subject, audited_until)
		 VALUES (?, ?, NOW(6) + INTERVAL ? MICROSECOND)`, scope, subject, us)
	if err != nil {
		return false, fmt.Errorf("claim throttle audit: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
