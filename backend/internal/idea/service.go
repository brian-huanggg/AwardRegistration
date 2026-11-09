// Package idea owns the idea table and is the only way to read an Idea
// (Backend.md 8): every read goes through the visibility query.
package idea

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Idea is the module's read model of one Idea.
type Idea = store.Idea

// Service reads and creates Ideas.
type Service struct {
	DB    *sql.DB
	Audit *audit.Service
	// Gate, Slides and Objects are needed by Create only.
	Gate    deadline.SubmissionGate
	Slides  SlideRules
	Objects ObjectStore
}

// Get returns the Idea if the caller may see it. A missing Idea and a hidden
// one both return the same NOT_FOUND error; the handler audits it as DENIED.
func (s *Service) Get(ctx context.Context, id uuid.UUID, caller security.Principal) (Idea, error) {
	got, err := store.GetVisible(ctx, s.DB, id, caller.UserID, caller.IsAdmin())
	if errors.Is(err, store.ErrNotFound) {
		return Idea{}, web.NewError(web.CodeNotFound, "")
	}
	return got, err
}

// Summary is the module's read model of one list row.
type Summary = store.Summary

// ListFilter is the validated filter set of the list.
type ListFilter = store.ListFilter

// List returns one page of the Ideas the caller may see and the total match
// count. limit and offset are already validated by the handler.
func (s *Service) List(ctx context.Context, caller security.Principal, f ListFilter, limit, offset int) ([]Summary, int, error) {
	return store.ListVisible(ctx, s.DB, caller.UserID, caller.IsAdmin(), f, limit, offset)
}
