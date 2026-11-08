// Package db holds the database helpers shared by every module: the
// transaction wrapper with its outcome classification, MySQL error helpers and
// the INET6 types.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Outcome says what is known about a transaction after InTx returns. Callers
// that touched external storage (MinIO) decide on compensation from it
// (ADR G7, Sequence.md §2).
type Outcome int

const (
	// Committed: COMMIT succeeded.
	Committed Outcome = iota
	// RolledBack: the transaction is known not to have been applied.
	RolledBack
	// Unknown: COMMIT failed in a way that may have been applied. Keep storage
	// objects; cleanup goes through pending_file_deletion.
	Unknown
)

func (o Outcome) String() string {
	switch o {
	case Committed:
		return "Committed"
	case RolledBack:
		return "RolledBack"
	case Unknown:
		return "Unknown"
	default:
		return fmt.Sprintf("Outcome(%d)", int(o))
	}
}

// TxTimeout bounds one whole transaction (ADR G8).
const TxTimeout = 10 * time.Second

// txTimeout is TxTimeout; tests shorten it.
var txTimeout = TxTimeout

// InTx runs fn in a READ COMMITTED transaction.
//
// The transaction runs on context.WithoutCancel(ctx) with its own timeout:
// database/sql rolls a transaction back when its context is cancelled, and a
// client disconnect must not turn into a rollback after an object was already
// uploaded (ADR G8). Values on ctx (request ID, principal) are kept.
//
// Outcome (ADR G7):
//   - BeginTx fails, or fn returns an error or panics: RolledBack. Rollback is
//     always called explicitly, because MariaDB rolls back only the statement
//     on a lock-wait timeout (1205).
//   - Commit fails with SQLState class 23 or 40: RolledBack.
//   - Commit fails any other way: Unknown.
//
// A lock-wait timeout (1205) or deadlock (1213) returned by fn, Begin or
// Commit is reported as a 503 *web.Error (SERVICE_UNAVAILABLE) wrapping the
// driver error.
func InTx(ctx context.Context, db *sql.DB, fn func(ctx context.Context, tx *sql.Tx) error) (Outcome, error) {
	txCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), txTimeout)
	defer cancel()

	tx, err := db.BeginTx(txCtx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return RolledBack, classify(err)
	}

	if err := runFn(txCtx, tx, fn); err != nil {
		// A failed Rollback cannot make the outcome worse: the transaction was
		// never committed, and the server drops it with the connection.
		_ = tx.Rollback()
		return RolledBack, classify(err)
	}

	if err := tx.Commit(); err != nil {
		switch sqlStateClass(err) {
		case "23", "40":
			return RolledBack, classify(err)
		default:
			return Unknown, classify(err)
		}
	}
	return Committed, nil
}

// runFn calls fn and rolls back before re-panicking, so a panic never leaves
// a transaction (and its locks) open on a pooled connection.
func runFn(ctx context.Context, tx *sql.Tx, fn func(context.Context, *sql.Tx) error) error {
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()
	return fn(ctx, tx)
}

// classify maps lock contention to 503 and leaves every other error as is.
func classify(err error) error {
	if !IsContention(err) {
		return err
	}
	e := web.NewError(web.CodeServiceUnavailable, "the database is busy, retry later")
	e.Cause = err
	return e
}
