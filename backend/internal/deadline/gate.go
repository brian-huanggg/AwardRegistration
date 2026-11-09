package deadline

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// SubmissionGate is the one place that decides whether a state-limited write
// may proceed (PRD 5, D34, OI-18).
type SubmissionGate interface {
	// Precheck is read-only. It runs before the request body is read, to keep
	// the error precedence of PRD 6.1, and never replaces AssertOpenInTx.
	Precheck(ctx context.Context, actor security.Principal) error
	// AssertOpenInTx is the authoritative check. The tx parameter makes it
	// impossible to call outside a write transaction (ADR G9).
	AssertOpenInTx(ctx context.Context, tx *sql.Tx, actor security.Principal) error
}

// Gate implements SubmissionGate. Admins are exempt (PRD 5, OI-18).
type Gate struct{ Store Store }

var _ SubmissionGate = Gate{}

// Precheck reads the deadline outside any transaction.
func (g Gate) Precheck(ctx context.Context, actor security.Principal) error {
	if actor.IsAdmin() {
		return nil
	}
	snap, err := g.Store.Snapshot(ctx)
	if err != nil {
		return err
	}
	return check(snap.State())
}

// AssertOpenInTx reads the deadline and NOW(6) in one statement on tx, without
// a lock (PRD 5): a write racing a deadline change may land on either side.
func (g Gate) AssertOpenInTx(ctx context.Context, tx *sql.Tx, actor security.Principal) error {
	if actor.IsAdmin() {
		return nil
	}
	snap, err := TxStore{Tx: tx}.Snapshot(ctx)
	if err != nil {
		return err
	}
	return check(snap.State())
}

// TxStore reads the deadline through an open transaction.
type TxStore struct{ Tx *sql.Tx }

// Snapshot returns the deadline and NOW(6) from one statement.
func (s TxStore) Snapshot(ctx context.Context) (Snapshot, error) {
	var snap Snapshot
	var d sql.NullTime
	err := s.Tx.QueryRowContext(ctx, `SELECT deadline, NOW(6) FROM contest_setting WHERE id = 1`).Scan(&d, &snap.Now)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read contest_setting: %w", err)
	}
	if d.Valid {
		snap.Deadline = &d.Time
	}
	return snap, nil
}

// check maps a state to the 403 a User write gets in it.
func check(s State) error {
	switch s {
	case "OPEN":
		return nil
	case "NOT_OPEN":
		return web.NewError(web.CodeSubmissionNotOpen, "submissions are not open yet")
	default:
		return web.NewError(web.CodeSubmissionClosed, "the submission deadline has passed")
	}
}
