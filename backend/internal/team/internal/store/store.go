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

// PendingRow is one PENDING row of a user with the minimal Idea information a
// JoinRequest carries (PRD 6.7).
type PendingRow struct {
	IdeaID                             uuid.UUID
	IdeaName, Category, Org, Direction string
	LeaderEmpNo, LeaderName            string
	RequestedAt                        time.Time
}

// ListPending returns the user's PENDING rows, newest first. Reads only; the
// row's created_at is the request time.
func ListPending(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, userID uuid.UUID) ([]PendingRow, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT i.id, i.name, i.category, i.org, i.direction, u.emp_no, u.name, m.created_at
		   FROM idea_member m
		   JOIN idea i ON i.id = m.idea_id
		   JOIN app_user u ON u.id = i.leader_id
		  WHERE m.user_id = ? AND m.status = 'PENDING'
		  ORDER BY m.created_at DESC, i.id`, userID.String())
	if err != nil {
		return nil, fmt.Errorf("select pending join requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []PendingRow{}
	for rows.Next() {
		var p PendingRow
		var id string
		if err := rows.Scan(&id, &p.IdeaName, &p.Category, &p.Org, &p.Direction,
			&p.LeaderEmpNo, &p.LeaderName, &p.RequestedAt); err != nil {
			return nil, fmt.Errorf("scan pending join request: %w", err)
		}
		if p.IdeaID, err = uuid.Parse(id); err != nil {
			return nil, fmt.Errorf("idea id: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending join requests: %w", err)
	}
	return out, nil
}

// Locked is the idea_member row of an applicant addressed by employee ID.
type Locked struct {
	UserID uuid.UUID
	Status string
}

// LockByEmpNo locks the row of the User with this employee ID on the Idea and
// returns it, or ErrNoRow. The caller already holds the Idea row lock.
func LockByEmpNo(ctx context.Context, tx *sql.Tx, ideaID uuid.UUID, empNo string) (Locked, error) {
	var (
		out Locked
		uid string
	)
	err := tx.QueryRowContext(ctx,
		`SELECT m.user_id, m.status FROM idea_member m JOIN app_user u ON u.id = m.user_id
		  WHERE m.idea_id = ? AND u.emp_no = ? FOR UPDATE`, ideaID.String(), empNo).Scan(&uid, &out.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return Locked{}, ErrNoRow
	}
	if err != nil {
		return Locked{}, fmt.Errorf("lock idea_member: %w", err)
	}
	if out.UserID, err = uuid.Parse(uid); err != nil {
		return Locked{}, fmt.Errorf("idea_member user id: %w", err)
	}
	return out, nil
}

// CountAccepted returns the number of approved Members of the Idea.
func CountAccepted(ctx context.Context, tx *sql.Tx, ideaID uuid.UUID) (int, error) {
	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM idea_member WHERE idea_id = ? AND status = 'ACCEPTED'`,
		ideaID.String()).Scan(&n); err != nil {
		return 0, fmt.Errorf("count accepted members: %w", err)
	}
	return n, nil
}

// Accept turns a PENDING row into ACCEPTED and sets accepted_at (database
// clock).
func Accept(ctx context.Context, tx *sql.Tx, ideaID, userID uuid.UUID) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE idea_member SET status = 'ACCEPTED', accepted_at = CURRENT_TIMESTAMP(6)
		  WHERE idea_id = ? AND user_id = ? AND status = 'PENDING'`,
		ideaID.String(), userID.String()); err != nil {
		return fmt.Errorf("accept idea_member: %w", err)
	}
	return nil
}

// Delete removes the pair's row (reject, remove, withdraw and leave delete the
// row; history lives in the Audit Log).
func Delete(ctx context.Context, tx *sql.Tx, ideaID, userID uuid.UUID) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM idea_member WHERE idea_id = ? AND user_id = ?`,
		ideaID.String(), userID.String()); err != nil {
		return fmt.Errorf("delete idea_member: %w", err)
	}
	return nil
}
