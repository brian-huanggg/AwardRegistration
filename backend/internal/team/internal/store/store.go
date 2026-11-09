// Package store holds the SQL of the team module: the idea_member table.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status of an idea_member row (V010).
const (
	Pending  = "PENDING"
	Accepted = "ACCEPTED"
)

// ErrNoRow is returned when the (Idea, User) pair has no row.
var ErrNoRow = errors.New("idea_member row not found")

// InsertPending inserts a PENDING row in the caller's transaction and returns
// its created_at (database clock). A primary-key violation is returned as the
// driver error (the pair already has a row); a missing Idea is a
// foreign-key error.
func InsertPending(ctx context.Context, tx *sql.Tx, ideaID, userID uuid.UUID) (time.Time, error) {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO idea_member (idea_id, user_id, status) VALUES (?, ?, 'PENDING')`,
		ideaID.String(), userID.String()); err != nil {
		return time.Time{}, fmt.Errorf("insert idea_member: %w", err)
	}
	var at time.Time
	if err := tx.QueryRowContext(ctx,
		`SELECT created_at FROM idea_member WHERE idea_id = ? AND user_id = ?`,
		ideaID.String(), userID.String()).Scan(&at); err != nil {
		return time.Time{}, fmt.Errorf("select idea_member created_at: %w", err)
	}
	return at, nil
}

// Status returns the status of the pair's row, or ErrNoRow.
func Status(ctx context.Context, tx *sql.Tx, ideaID, userID uuid.UUID) (string, error) {
	var st string
	err := tx.QueryRowContext(ctx,
		`SELECT status FROM idea_member WHERE idea_id = ? AND user_id = ?`,
		ideaID.String(), userID.String()).Scan(&st)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoRow
	}
	if err != nil {
		return "", fmt.Errorf("select idea_member status: %w", err)
	}
	return st, nil
}
