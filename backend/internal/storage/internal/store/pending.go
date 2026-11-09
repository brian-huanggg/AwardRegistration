// Package store holds the SQL of the storage module (pending_file_deletion,
// V008). Only the storage module may import it.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Execer is satisfied by *sql.DB, *sql.Tx and *sql.Conn.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// InsertPending records key for deletion; the job may act on it from database
// time NOW + delay. Returns the row id.
func InsertPending(ctx context.Context, q Execer, key, reason string, delay time.Duration) (int64, error) {
	res, err := q.ExecContext(ctx,
		`INSERT INTO pending_file_deletion (object_key, reason, next_attempt_at)
		 VALUES (?, ?, NOW(6) + INTERVAL ? MICROSECOND)`,
		key, reason, delay.Microseconds())
	if err != nil {
		return 0, fmt.Errorf("insert pending_file_deletion: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert pending_file_deletion: %w", err)
	}
	return id, nil
}

// DeletePending removes one row after its object was deleted. A missing row
// is not an error.
func DeletePending(ctx context.Context, q Execer, id int64) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM pending_file_deletion WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete pending_file_deletion %d: %w", id, err)
	}
	return nil
}
