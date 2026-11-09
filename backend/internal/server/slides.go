package server

import (
	"context"
	"database/sql"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
)

// IdeaSlideRules adapts the storage slide rules to idea.SlideRules. It lives
// here because idea must not import storage (D29) and storage must not know
// idea's types.
func IdeaSlideRules() idea.SlideRules {
	return idea.SlideRules{
		ExpectedBase: storage.ExpectedFilenameBase,
		CheckLength:  storage.CheckFilenameLength,
		Validate: func(f idea.SlideFile, expectedBase string) (string, error) {
			// ReadMultipart guarantees exactly one part (step 1, 400).
			kind, err := storage.Validate([]storage.Part{{Filename: f.Filename, Size: f.Size, Data: f.Data}}, expectedBase)
			return string(kind), err
		},
	}
}

// IdeaReplacedKeys adapts storage.PendingDeletion to idea.ReplacedKeys.
type IdeaReplacedKeys struct{ P *storage.PendingDeletion }

// RecordReplaced records key as REPLACED inside tx.
func (r IdeaReplacedKeys) RecordReplaced(ctx context.Context, tx *sql.Tx, key string) (int64, error) {
	id, err := r.P.RecordInTx(ctx, tx, key, storage.ReasonReplaced)
	return int64(id), err
}

// DeleteAfterCommit deletes the old object and its pending row, best effort.
func (r IdeaReplacedKeys) DeleteAfterCommit(ctx context.Context, id int64, key string) {
	r.P.DeleteAfterCommit(ctx, storage.PendingID(id), key)
}

// IdeaDeletedKeys adapts storage.PendingDeletion to idea.DeletedKeys.
type IdeaDeletedKeys struct{ P *storage.PendingDeletion }

// RecordDeleted records key as DELETED inside tx.
func (r IdeaDeletedKeys) RecordDeleted(ctx context.Context, tx *sql.Tx, key string) (int64, error) {
	id, err := r.P.RecordInTx(ctx, tx, key, storage.ReasonDeleted)
	return int64(id), err
}

// DeleteAfterCommit deletes the object and its pending row, best effort.
func (r IdeaDeletedKeys) DeleteAfterCommit(ctx context.Context, id int64, key string) {
	r.P.DeleteAfterCommit(ctx, storage.PendingID(id), key)
}
