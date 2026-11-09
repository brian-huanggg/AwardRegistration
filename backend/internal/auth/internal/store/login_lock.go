package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// LockPolicy is the key-lock rule (PRD §6.5), in seconds and failures.
type LockPolicy struct {
	MaxFailures   int
	WindowSeconds int // a failure older than this no longer counts as consecutive
	LockSeconds   int // fixed lock length
}

// RecordFailure counts one failed attempt for key in ONE statement on database
// time. The assignments run left to right in MariaDB, so later columns see the
// new fail_count. A lock that is still active is never extended or re-counted
// (a concurrent attempt that passed Check before the lock was set); an expired
// lock or a stale window starts again at 1.
func RecordFailure(ctx context.Context, db *sql.DB, key string, p LockPolicy) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO login_lock (lock_key, fail_count, last_fail_at, locked_until)
		 VALUES (?, 1, NOW(6), NULL)
		 ON DUPLICATE KEY UPDATE
		   fail_count = IF(locked_until > NOW(6), fail_count,
		                   IF(locked_until IS NOT NULL OR last_fail_at <= NOW(6) - INTERVAL ? SECOND, 1, fail_count + 1)),
		   locked_until = CASE WHEN locked_until > NOW(6) THEN locked_until
		                       WHEN fail_count >= ? THEN NOW(6) + INTERVAL ? SECOND
		                       ELSE NULL END,
		   last_fail_at = NOW(6)`,
		key, p.WindowSeconds, p.MaxFailures, p.LockSeconds)
	if err != nil {
		return fmt.Errorf("record login failure: %w", err)
	}
	return nil
}

// LockRemaining returns how long key stays locked, 0 when it is not locked.
func LockRemaining(ctx context.Context, db *sql.DB, key string) (time.Duration, error) {
	var us sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT TIMESTAMPDIFF(MICROSECOND, NOW(6), locked_until)
		   FROM login_lock WHERE lock_key = ? AND locked_until > NOW(6)`, key).Scan(&us)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read login lock: %w", err)
	}
	return time.Duration(us.Int64) * time.Microsecond, nil
}

// ClearFailures resets the counter after a successful login. An active lock is
// left alone (a success cannot shorten it).
func ClearFailures(ctx context.Context, db *sql.DB, key string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM login_lock WHERE lock_key = ? AND (locked_until IS NULL OR locked_until <= NOW(6))`, key)
	if err != nil {
		return fmt.Errorf("clear login failures: %w", err)
	}
	return nil
}

// DeleteStaleLocks removes at most limit login_lock rows that no longer
// matter: no active lock and the last failure outside the window. For such a
// row RecordFailure would start again at 1, exactly as for a missing row.
func DeleteStaleLocks(ctx context.Context, db *sql.DB, p LockPolicy, limit int) (int64, error) {
	return deleteRows(ctx, db, "purge login locks",
		`DELETE FROM login_lock
		  WHERE last_fail_at <= NOW(6) - INTERVAL ? SECOND
		    AND (locked_until IS NULL OR locked_until <= NOW(6))
		  LIMIT ?`,
		p.WindowSeconds, limit)
}
