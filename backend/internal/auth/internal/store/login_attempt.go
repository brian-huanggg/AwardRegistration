package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// IPPolicy is the per-IP sliding-window rule (PRD §6.5).
type IPPolicy struct {
	MaxFailures   int
	WindowSeconds int
}

// RecordIPFailure stores one failed attempt of ip at database time.
func RecordIPFailure(ctx context.Context, db *sql.DB, ip string) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO login_attempt (ip, attempted_at) VALUES (?, NOW(6))`, ip); err != nil {
		return fmt.Errorf("record login attempt: %w", err)
	}
	return nil
}

// IPRetryAfter returns how long ip stays over the limit, 0 when it is not.
// The address is over the limit while at least MaxFailures failures lie inside
// the window; it is free again when the MaxFailures-th newest one leaves it,
// so that row's age decides (with exactly MaxFailures rows this is the oldest).
func IPRetryAfter(ctx context.Context, db *sql.DB, ip string, p IPPolicy) (time.Duration, error) {
	var us int64
	err := db.QueryRowContext(ctx,
		`SELECT TIMESTAMPDIFF(MICROSECOND, NOW(6), attempted_at + INTERVAL ? SECOND)
		   FROM login_attempt
		  WHERE ip = ? AND attempted_at > NOW(6) - INTERVAL ? SECOND
		  ORDER BY attempted_at DESC
		  LIMIT 1 OFFSET ?`, p.WindowSeconds, ip, p.WindowSeconds, p.MaxFailures-1).Scan(&us)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read login attempts: %w", err)
	}
	return time.Duration(us) * time.Microsecond, nil
}
