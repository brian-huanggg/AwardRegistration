package storage

import (
	"context"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
)

// Misuse is rejected before any database call (nil pool on purpose).
func TestPendingDeletionRejectsMisuse(t *testing.T) {
	p := NewPendingDeletion(nil, nil, nil)
	ctx := context.Background()
	if _, err := p.RecordInTx(ctx, nil, "k", ReasonUploadAborted); err == nil {
		t.Error("RecordInTx(UPLOAD_ABORTED): want error, the key is recorded outside the tx")
	}
	if _, err := p.RecordInTx(ctx, nil, "k", Reason("OTHER")); err == nil {
		t.Error("RecordInTx(OTHER): want error")
	}
	if err := p.RecordUploadAborted(ctx, "k", db.Committed); err == nil {
		t.Error("RecordUploadAborted(Committed): want error, a committed key is referenced")
	}
}
