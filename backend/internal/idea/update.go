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

// UpdateCommand is the raw, unvalidated edit form. The Idea's Leader is the caller.
type UpdateCommand struct {
	Category, Org, Direction, Name, Summary string
}

// markDenied marks a 404, role or state refusal of an edit for the DENIED audit
// row (track B, written by the central error adapter). Other errors pass through.
func markDenied(err error, action audit.Action, id uuid.UUID, reason string) error {
	var e *web.Error
	if errors.As(err, &e) && (e.Code == web.CodeNotFound || e.Code == web.CodeForbidden ||
		e.Code == web.CodeSubmissionNotOpen || e.Code == web.CodeSubmissionClosed) {
		e.AuditAction = string(action)
		e.AuditReason = reason
		e.AuditTargetType = string(audit.TargetIdea)
		e.AuditTargetID = id
	}
	return err
}

// AuthorizeEdit runs the checks that come before the body is read (PRD 6.1,
// Backend.md §3): 404 for a missing or hidden Idea → 403 FORBIDDEN for a
// viewer who is not the Leader (Admin, Member) → 403 SUBMISSION_* by the
// read-only state pre-check. Each refusal is audited as DENIED.
func (s *Service) AuthorizeEdit(ctx context.Context, id uuid.UUID, caller security.Principal) error {
	got, err := s.Get(ctx, id, caller)
	if err != nil {
		return markDenied(err, audit.IdeaUpdate, id, auditReasonHidden)
	}
	if got.LeaderID != caller.UserID {
		return markDenied(web.NewError(web.CodeForbidden, "Only the Leader can edit an Idea."),
			audit.IdeaUpdate, id, auditReasonRole)
	}
	return markDenied(s.Gate.Precheck(ctx, caller), audit.IdeaUpdate, id, auditReasonState)
}

// Update runs PUT /ideas/{id} after AuthorizeEdit (done by the handler before
// the body is read). Order: text 422 (incl. FILENAME_TOO_LONG) → transaction:
// lock row, leader and AssertOpenInTx again, UPDATE, IDEA_UPDATE with the
// changed fields only, read back. A duplicate name is found by the unique key,
// which stays authoritative: there is no upload to save, so no pre-check.
// A request that changes nothing writes the audit row but no UPDATE, so the
// version stays. An unknown transaction outcome answers 503.
func (s *Service) Update(ctx context.Context, id uuid.UUID, caller security.Principal, cmd UpdateCommand, req Request) (Idea, error) {
	leaderName, err := store.LeaderName(ctx, s.DB, caller.UserID)
	if err != nil {
		return Idea{}, err
	}
	v, _, err := s.validateText(CreateCommand{
		Category: cmd.Category, Org: cmd.Org, Direction: cmd.Direction, Name: cmd.Name, Summary: cmd.Summary,
	}, leaderName, caller.EmpNo)
	if err != nil {
		return Idea{}, err
	}
	next := store.Text{Category: v.category, Org: v.org, Direction: v.direction, Name: v.name, Summary: v.summary}

	var got Idea
	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		leader, cur, err := store.LockText(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			return markDenied(web.NewError(web.CodeNotFound, ""), audit.IdeaUpdate, id, auditReasonHidden)
		}
		if err != nil {
			return err
		}
		if leader != caller.UserID {
			return markDenied(web.NewError(web.CodeForbidden, "Only the Leader can edit an Idea."),
				audit.IdeaUpdate, id, auditReasonRole)
		}
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return markDenied(err, audit.IdeaUpdate, id, auditReasonState)
		}
		before, after, err := changedFields(cur, next)
		if err != nil {
			return err
		}
		if cur != next {
			if err := store.UpdateText(ctx, tx, id, next); err != nil {
				return err
			}
		}
		if err := s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.IdeaUpdate, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
			Method: req.Method, Route: req.Route, Status: http.StatusOK,
			IP: req.IP, UserAgent: req.UserAgent,
			Before: before, After: after,
		}); err != nil {
			return err
		}
		got, err = store.GetVisible(ctx, tx, id, caller.UserID, caller.IsAdmin())
		return err
	})
	switch out {
	case db.Committed:
		return got, nil
	case db.Unknown:
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check your Idea before retrying.")
		e.Cause = err
		return Idea{}, e
	case db.RolledBack:
	}
	if _, dup := db.DuplicateKey(err); dup {
		return Idea{}, duplicateErr(err)
	}
	return Idea{}, err
}

// changedFields returns the IDEA_UPDATE before and after documents: only the
// fields whose value differs (PRD 6.11). Both are "{}" when nothing changed.
func changedFields(cur, next store.Text) (before, after json.RawMessage, err error) {
	b, a := map[string]string{}, map[string]string{}
	for _, f := range []struct{ name, old, new string }{
		{fieldCategory, cur.Category, next.Category},
		{fieldOrg, cur.Org, next.Org},
		{fieldDirection, cur.Direction, next.Direction},
		{fieldName, cur.Name, next.Name},
		{fieldSummary, cur.Summary, next.Summary},
	} {
		if f.old != f.new {
			b[f.name], a[f.name] = f.old, f.new
		}
	}
	if before, err = json.Marshal(b); err != nil {
		return nil, nil, fmt.Errorf("marshal IDEA_UPDATE before: %w", err)
	}
	if after, err = json.Marshal(a); err != nil {
		return nil, nil, fmt.Errorf("marshal IDEA_UPDATE after: %w", err)
	}
	return before, after, nil
}

// maxJSONBody is the JSON body limit (error-codes.md §2: 16 KiB → 413).
const maxJSONBody = 16 << 10

// updateBody holds pointers so DecodeJSON can tell absent/null (400) from
// empty (422). Unknown keys are ignored.
type updateBody struct {
	Category  *string `json:"category"`
	Org       *string `json:"org"`
	Direction *string `json:"direction"`
	Name      *string `json:"name"`
	Summary   *string `json:"summary"`
}

// UpdateIdea serves PUT /ideas/{id}: 200 + IdeaDetail. 404, role and state
// run before the body is read (Backend.md §3).
func (h Handler) UpdateIdea(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	if err := h.Service.AuthorizeEdit(r.Context(), id, p); err != nil {
		h.Fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var b updateBody
	if err := web.DecodeJSON(r, &b); err != nil {
		h.Fail(w, r, err)
		return
	}
	got, err := h.Service.Update(r.Context(), id, p, UpdateCommand{
		Category: *b.Category, Org: *b.Org, Direction: *b.Direction, Name: *b.Name, Summary: *b.Summary,
	}, Request{Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r)})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	body, err := h.toDetail(got, p)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, body)
}
