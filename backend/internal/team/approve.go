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

const approveForbidden = "Only the Leader can approve a join request."

// markDeniedFor is markDenied for an action on a known Idea: the DENIED row
// names the Idea as its target.
func markDeniedFor(err error, action audit.Action, id uuid.UUID, reason string) error {
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

// Approve runs POST /ideas/{id}/members/{empNo}/approve. Order (PRD 6.1, 6.7):
// 404 for a missing or hidden Idea → 403 FORBIDDEN for a viewer who is not the
// Leader (Admin, Member) → 403 SUBMISSION_* by the read-only pre-check →
// transaction: lock the Idea row, Leader and AssertOpenInTx again, lock the
// applicant's row (none → 404; not PENDING → 409 JOIN_REQUEST_NOT_PENDING),
// count ACCEPTED rows (3 → 409 TEAM_FULL), set ACCEPTED + accepted_at,
// JOIN_REQUEST_APPROVE. The Idea row lock serializes concurrent approvals, so
// capacity never exceeds 3. An unknown transaction outcome answers 503.
func (s *Service) Approve(ctx context.Context, id uuid.UUID, empNo string, caller security.Principal, req Request) error {
	const action = audit.JoinRequestApprove
	got, err := s.Ideas.Get(ctx, id, caller)
	if err != nil {
		return markDeniedFor(err, action, id, auditReasonHidden)
	}
	if got.LeaderID != caller.UserID {
		return markDeniedFor(web.NewError(web.CodeForbidden, approveForbidden), action, id, auditReasonRole)
	}
	if err := markDeniedFor(s.Gate.Precheck(ctx, caller), action, id, auditReasonState); err != nil {
		return err
	}

	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		leader, err := s.Ideas.LockLeader(ctx, tx, id)
		if err != nil {
			return markDeniedFor(err, action, id, auditReasonHidden)
		}
		if leader != caller.UserID {
			return markDeniedFor(web.NewError(web.CodeForbidden, approveForbidden), action, id, auditReasonRole)
		}
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return markDeniedFor(err, action, id, auditReasonState)
		}
		row, err := store.LockByEmpNo(ctx, tx, id, empNo)
		if errors.Is(err, store.ErrNoRow) {
			return markDeniedFor(web.NewError(web.CodeNotFound, ""), action, id, auditReasonHidden)
		}
		if err != nil {
			return err
		}
		if row.Status != store.Pending {
			return web.NewError(web.CodeJoinRequestNotPend, "This join request is not pending.")
		}
		n, err := store.CountAccepted(ctx, tx, id)
		if err != nil {
			return err
		}
		if n >= maxMembers {
			return web.NewError(web.CodeTeamFull, "This team is full.")
		}
		if err := store.Accept(ctx, tx, id, row.UserID); err != nil {
			return err
		}
		before, err := json.Marshal(map[string]any{"empNo": empNo, "status": store.Pending})
		if err != nil {
			return fmt.Errorf("marshal JOIN_REQUEST_APPROVE before: %w", err)
		}
		after, err := json.Marshal(map[string]any{"empNo": empNo, "status": store.Accepted})
		if err != nil {
			return fmt.Errorf("marshal JOIN_REQUEST_APPROVE after: %w", err)
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: action, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
			Method: req.Method, Route: req.Route, Status: http.StatusOK,
			IP: req.IP, UserAgent: req.UserAgent,
			Before: before, After: after,
		})
	})
	if out == db.Unknown {
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check the Idea before retrying.")
		e.Cause = err
		return e
	}
	return err
}
