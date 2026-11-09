package idea

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// DeletedKeys records and removes the object of a deleted Idea
// (storage.PendingDeletion, adapted by main because idea must not import
// storage, D29). RecordDeleted runs inside the delete transaction, so the row
// exists exactly when the Idea is gone. DeleteAfterCommit is best effort and
// never fails the request.
type DeletedKeys interface {
	RecordDeleted(ctx context.Context, tx *sql.Tx, key string) (id int64, err error)
	DeleteAfterCommit(ctx context.Context, id int64, key string)
}

const deleteForbidden = "Only the Leader or an Admin can delete an Idea."

// Delete runs DELETE /ideas/{id}. Order: 404 for a missing or hidden Idea →
// 403 FORBIDDEN for a viewer who is neither Leader nor Admin (Member) → 403
// SUBMISSION_* by the read-only state pre-check (Admin is exempt) →
// transaction: lock row, leader/Admin and AssertOpenInTx again, DELETE (member
// rows cascade), DELETED row, IDEA_DELETE. After commit the object is deleted
// best effort, so a delete succeeds with MinIO down. An unknown transaction
// outcome answers 503; nothing has been touched in storage then.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, caller security.Principal, req Request) error {
	got, err := s.Get(ctx, id, caller)
	if err != nil {
		return markDenied(err, audit.IdeaDelete, id, auditReasonHidden)
	}
	if !caller.IsAdmin() && got.LeaderID != caller.UserID {
		return markDenied(web.NewError(web.CodeForbidden, deleteForbidden), audit.IdeaDelete, id, auditReasonRole)
	}
	if err := markDenied(s.Gate.Precheck(ctx, caller), audit.IdeaDelete, id, auditReasonState); err != nil {
		return err
	}

	var (
		key       string
		pendingID int64
	)
	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		leader, text, err := store.LockText(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			return markDenied(web.NewError(web.CodeNotFound, ""), audit.IdeaDelete, id, auditReasonHidden)
		}
		if err != nil {
			return err
		}
		if !caller.IsAdmin() && leader != caller.UserID {
			return markDenied(web.NewError(web.CodeForbidden, deleteForbidden), audit.IdeaDelete, id, auditReasonRole)
		}
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return markDenied(err, audit.IdeaDelete, id, auditReasonState)
		}
		_, slides, err := store.LockSlides(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := store.DeleteIdea(ctx, tx, id); err != nil {
			return err
		}
		if pendingID, err = s.Deleted.RecordDeleted(ctx, tx, slides.ObjectKey); err != nil {
			return err
		}
		key = slides.ObjectKey
		before, err := deleteDoc(text, slides)
		if err != nil {
			return err
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.IdeaDelete, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
			Method: req.Method, Route: req.Route, Status: http.StatusNoContent,
			IP: req.IP, UserAgent: req.UserAgent,
			Before: before,
		})
	})
	switch out {
	case db.Committed:
		s.Deleted.DeleteAfterCommit(ctx, pendingID, key)
		return nil
	case db.Unknown:
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check your Idea before retrying.")
		e.Cause = err
		return e
	case db.RolledBack:
	}
	return err
}

// deleteDoc is the IDEA_DELETE before document: the text fields and slide
// metadata, never the content (PRD 6.11).
func deleteDoc(t store.Text, s store.Slides) (json.RawMessage, error) {
	b, err := json.Marshal(map[string]any{
		fieldCategory: t.Category, fieldOrg: t.Org, fieldDirection: t.Direction,
		fieldName: t.Name, fieldSummary: t.Summary,
		"slides": map[string]any{"ext": s.Ext, "size": s.Size, "sha256": s.SHA256},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal IDEA_DELETE before: %w", err)
	}
	return b, nil
}

// DeleteIdea serves DELETE /ideas/{id}: 204.
func (h Handler) DeleteIdea(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	err := h.Service.Delete(r.Context(), id, p,
		Request{Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r)})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
