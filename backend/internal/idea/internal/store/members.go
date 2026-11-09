package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Person is a Member or an applicant as the Idea detail lists them; At is
// accepted_at for a Member and created_at for an applicant.
type Person struct {
	EmpNo, Name string
	At          time.Time
}

type rowsQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Members returns the ACCEPTED Members of an Idea, oldest approval first.
func Members(ctx context.Context, q rowsQuerier, id uuid.UUID) ([]Person, error) {
	return people(ctx, q, id, `m.status = 'ACCEPTED'`, `m.accepted_at`)
}

// Pending returns the PENDING applicants of an Idea, oldest request first.
func Pending(ctx context.Context, q rowsQuerier, id uuid.UUID) ([]Person, error) {
	return people(ctx, q, id, `m.status = 'PENDING'`, `m.created_at`)
}

func people(ctx context.Context, q rowsQuerier, id uuid.UUID, where, at string) ([]Person, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT u.emp_no, u.name, `+at+`
		   FROM idea_member m JOIN app_user u ON u.id = m.user_id
		  WHERE m.idea_id = ? AND `+where+` ORDER BY `+at+`, u.emp_no`, id.String())
	if err != nil {
		return nil, fmt.Errorf("select idea people: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Person{}
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.EmpNo, &p.Name, &p.At); err != nil {
			return nil, fmt.Errorf("scan idea person: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate idea people: %w", err)
	}
	return out, nil
}

// LockLeader locks the idea row and returns its Leader. A missing row is
// ErrNotFound. It is the first statement of a team transaction, so
// concurrent approvals of one Idea serialize here.
func LockLeader(ctx context.Context, tx *sql.Tx, id uuid.UUID) (uuid.UUID, error) {
	var lid string
	err := tx.QueryRowContext(ctx, `SELECT leader_id FROM idea WHERE id = ? FOR UPDATE`, id.String()).Scan(&lid)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lock idea leader: %w", err)
	}
	leader, err := uuid.Parse(lid)
	if err != nil {
		return uuid.Nil, fmt.Errorf("idea leader id: %w", err)
	}
	return leader, nil
}
