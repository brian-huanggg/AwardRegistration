package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage/internal/store"
)

// Reason is why an object key is in pending_file_deletion (Sequence.md §2).
type Reason string

const (
	// ReasonReplaced: old key, recorded in the transaction that switched the pointer.
	ReasonReplaced Reason = "REPLACED"
	// ReasonDeleted: key of a deleted Idea, recorded in the delete transaction.
	ReasonDeleted Reason = "DELETED"
	// ReasonUploadAborted: new key whose transaction rolled back (and the
	// immediate delete failed) or ended with an unknown outcome.
	ReasonUploadAborted Reason = "UPLOAD_ABORTED"
)

// UploadAbortedGrace delays the clean-up of a key recorded after an unknown
// outcome: longer than the maximum lock wait plus transaction time, so a COMMIT
// the server is still finishing is never raced (D36). [Proposed] OI-19.
const UploadAbortedGrace = 10 * time.Minute

// bookkeepingTimeout bounds one write that runs outside the request's own
// transaction (separate UPLOAD_ABORTED insert, post-commit delete).
const bookkeepingTimeout = 5 * time.Second

// PendingID identifies a pending_file_deletion row.
type PendingID int64

// PendingDeletion writes pending_file_deletion (PRD §6.8, D25) and makes the
// best-effort post-commit delete. The table is drained by the clean-up job
// (next phase); nothing here waits for it.
type PendingDeletion struct {
	db    *sql.DB
	store SlideStore
	log   *slog.Logger
}

// NewPendingDeletion builds the service. log nil discards.
func NewPendingDeletion(database *sql.DB, s SlideStore, log *slog.Logger) *PendingDeletion {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &PendingDeletion{db: database, store: s, log: log}
}

// RecordInTx records key with REPLACED or DELETED inside the caller's
// transaction, so the row exists exactly when the change committed. After a
// Committed outcome the caller passes the id to DeleteAfterCommit.
func (p *PendingDeletion) RecordInTx(ctx context.Context, tx *sql.Tx, key string, reason Reason) (PendingID, error) {
	if reason != ReasonReplaced && reason != ReasonDeleted {
		return 0, fmt.Errorf("storage: RecordInTx with reason %q", reason)
	}
	id, err := store.InsertPending(ctx, tx, key, string(reason), 0)
	return PendingID(id), err
}

// RecordUploadAborted records a newly uploaded key whose transaction did not
// commit, in its own transaction (best effort; the request is failing anyway).
//
//   - RolledBack (the immediate delete failed): due at once.
//   - Unknown (the COMMIT may have been applied): due after UploadAbortedGrace;
//     the job keeps the object if an idea row references the key.
//
// Committed is a programming error: a committed key is referenced.
func (p *PendingDeletion) RecordUploadAborted(ctx context.Context, key string, outcome db.Outcome) error {
	var delay time.Duration
	switch outcome {
	case db.RolledBack:
	case db.Unknown:
		delay = UploadAbortedGrace
	default:
		return fmt.Errorf("storage: RecordUploadAborted after %v", outcome)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), bookkeepingTimeout)
	defer cancel()
	if _, err := store.InsertPending(ctx, p.db, key, string(ReasonUploadAborted), delay); err != nil {
		p.log.ErrorContext(ctx, "pending_file_deletion insert failed; object may be orphaned",
			slog.String("objectKey", key), slog.String("outcome", outcome.String()), slog.Any("error", err))
		return err
	}
	return nil
}

// DeleteAfterCommit deletes key from storage and then its pending row. It runs
// after the response outcome is fixed and never fails the request: a storage
// or database failure is logged and the row stays for the clean-up job
// (Sequence.md §4, §5). The caller's cancellation is ignored, so a client that
// disconnects does not skip it.
func (p *PendingDeletion) DeleteAfterCommit(ctx context.Context, id PendingID, key string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), CallTimeout+bookkeepingTimeout)
	defer cancel()
	if err := p.store.Delete(ctx, key); err != nil {
		p.log.WarnContext(ctx, "post-commit object delete failed; left for the clean-up job",
			slog.String("objectKey", key), slog.Int64("pendingId", int64(id)), slog.Any("error", err))
		return
	}
	if err := store.DeletePending(ctx, p.db, int64(id)); err != nil {
		// The object is gone; the job's delete of a missing key succeeds.
		p.log.WarnContext(ctx, "pending_file_deletion row not removed after delete",
			slog.String("objectKey", key), slog.Int64("pendingId", int64(id)), slog.Any("error", err))
	}
}
