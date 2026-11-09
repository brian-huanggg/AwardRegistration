package team

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/team/internal/store"
)

const removeForbidden = "Only the Leader can reject a join request or remove a Member."

// Remove runs DELETE /ideas/{id}/members/{empNo}: reject a PENDING request or
// remove an ACCEPTED Member (PRD 6.7). Order (PRD 6.1): 404 for a missing or
// hidden Idea → 403 FORBIDDEN for a viewer who is not the Leader (Admin,
// Member) → 403 SUBMISSION_* by the read-only pre-check → transaction: lock
// the Idea row, Leader and AssertOpenInTx again, lock the target row (none,
// including the Leader's own employee ID → 404), delete it and write
// JOIN_REQUEST_REJECT (was PENDING) or MEMBER_REMOVE (was ACCEPTED). A refusal
// before the row is known is audited as MEMBER_REMOVE. An unknown transaction
// outcome answers 503.
func (s *Service) Remove(ctx context.Context, id uuid.UUID, empNo string, caller security.Principal, req Request) error {
	const denied = audit.MemberRemove
	got, err := s.Ideas.Get(ctx, id, caller)
	if err != nil {
		return markDeniedFor(err, denied, id, auditReasonHidden)
	}
	if got.LeaderID != caller.UserID {
		return markDeniedFor(web.NewError(web.CodeForbidden, removeForbidden), denied, id, auditReasonRole)
	}
	if err := markDeniedFor(s.Gate.Precheck(ctx, caller), denied, id, auditReasonState); err != nil {
		return err
	}

	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		leader, err := s.Ideas.LockLeader(ctx, tx, id)
		if err != nil {
			return markDeniedFor(err, denied, id, auditReasonHidden)
		}
		if leader != caller.UserID {
			return markDeniedFor(web.NewError(web.CodeForbidden, removeForbidden), denied, id, auditReasonRole)
		}
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return markDeniedFor(err, denied, id, auditReasonState)
		}
		row, err := store.LockByEmpNo(ctx, tx, id, empNo)
		if errors.Is(err, store.ErrNoRow) {
			return markDeniedFor(web.NewError(web.CodeNotFound, ""), denied, id, auditReasonHidden)
		}
		if err != nil {
			return err
		}
		action := audit.MemberRemove
		if row.Status == store.Pending {
			action = audit.JoinRequestReject
		}
		if err := store.Delete(ctx, tx, id, row.UserID); err != nil {
			return err
		}
		before, err := json.Marshal(map[string]any{"empNo": empNo, "status": row.Status})
		if err != nil {
			return fmt.Errorf("marshal %s before: %w", action, err)
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: action, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
			Method: req.Method, Route: req.Route, Status: http.StatusNoContent,
			IP: req.IP, UserAgent: req.UserAgent,
			Before: before,
		})
	})
	if out == db.Unknown {
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check the Idea before retrying.")
		e.Cause = err
		return e
	}
	return err
}

// Leave runs DELETE /ideas/{id}/members/me: withdraw the caller's PENDING
// request or leave as an ACCEPTED Member (PRD 6.7). Order: a caller without a
// row on the Idea (stranger, Admin, the Leader, missing Idea) → uniform 404 →
// 403 SUBMISSION_* by the read-only pre-check → transaction: lock the Idea row
// (gone → 404), AssertOpenInTx again, lock the caller's own row (none → 404),
// delete it and write JOIN_REQUEST_WITHDRAW (was PENDING) or MEMBER_LEAVE (was
// ACCEPTED). The row check comes before the state check so a closed window
// never tells a stranger that the Idea exists. A refusal before the row is
// known is audited as MEMBER_LEAVE. An unknown transaction outcome answers 503.
func (s *Service) Leave(ctx context.Context, id uuid.UUID, caller security.Principal, req Request) error {
	const denied = audit.MemberLeave
	has, err := store.HasRow(ctx, s.DB, id, caller.UserID)
	if err != nil {
		return err
	}
	if !has {
		return markDeniedFor(web.NewError(web.CodeNotFound, ""), denied, id, auditReasonHidden)
	}
	if err := markDeniedFor(s.Gate.Precheck(ctx, caller), denied, id, auditReasonState); err != nil {
		return err
	}

	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := s.Ideas.LockLeader(ctx, tx, id); err != nil {
			return markDeniedFor(err, denied, id, auditReasonHidden)
		}
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return markDeniedFor(err, denied, id, auditReasonState)
		}
		status, err := store.LockOwn(ctx, tx, id, caller.UserID)
		if errors.Is(err, store.ErrNoRow) {
			return markDeniedFor(web.NewError(web.CodeNotFound, ""), denied, id, auditReasonHidden)
		}
		if err != nil {
			return err
		}
		action := audit.MemberLeave
		if status == store.Pending {
			action = audit.JoinRequestWithdraw
		}
		if err := store.Delete(ctx, tx, id, caller.UserID); err != nil {
			return err
		}
		before, err := json.Marshal(map[string]any{"status": status})
		if err != nil {
			return fmt.Errorf("marshal %s before: %w", action, err)
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: action, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
			Method: req.Method, Route: req.Route, Status: http.StatusNoContent,
			IP: req.IP, UserAgent: req.UserAgent,
			Before: before,
		})
	})
	if out == db.Unknown {
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check the Idea before retrying.")
		e.Cause = err
		return e
	}
	return err
}
