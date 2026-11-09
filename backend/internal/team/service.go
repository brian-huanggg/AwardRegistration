// Package team owns the idea_member table: join requests and Members
// (PRD 6.7). It resolves Ideas only through the idea module.
package team

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/text"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/team/internal/store"
)

// Limits and field names of a join request (PRD 6.3, 6.6, 6.7).
const (
	empNoLen     = 6
	maxIdeaName  = 75
	fieldLeader  = "leaderEmpNo"
	fieldIdeaNam = "ideaName"
)

// Audit reasons of a refused join request (DENIED rows, PRD 6.11).
const (
	auditReasonRole   = "ROLE"
	auditReasonState  = "STATE"
	auditReasonHidden = "NOT_FOUND"
)

// Ideas is the part of the idea module team needs.
type Ideas interface {
	FindByLeader(ctx context.Context, leaderEmpNo, name string) (idea.Idea, error)
}

// Service owns join requests and Members.
type Service struct {
	DB    *sql.DB
	Audit *audit.Service
	Gate  deadline.SubmissionGate
	Ideas Ideas
}

// Request is the transport facts the audit row needs.
type Request struct {
	Method, Route, UserAgent string
	IP                       netip.Addr
}

// JoinRequest is the minimal Idea information an applicant sees.
type JoinRequest struct {
	IdeaID                             uuid.UUID
	IdeaName, Category, Org, Direction string
	LeaderEmpNo, LeaderName            string
	RequestedAt                        time.Time
}

// markDenied marks a role, state or not-found refusal for the DENIED audit row
// (track B, written by the central error adapter). Other errors pass through.
func markDenied(err error, reason string) error {
	var e *web.Error
	if errors.As(err, &e) && (e.Code == web.CodeNotFound || e.Code == web.CodeForbidden ||
		e.Code == web.CodeSubmissionNotOpen || e.Code == web.CodeSubmissionClosed) {
		e.AuditAction = string(audit.JoinRequestCreate)
		e.AuditReason = reason
	}
	return err
}

// AuthorizeApply runs the checks that come before the body is read (PRD 6.1):
// role 403 (Admin) → state 403 by the read-only pre-check.
func (s *Service) AuthorizeApply(ctx context.Context, caller security.Principal) error {
	if caller.IsAdmin() {
		return markDenied(web.NewError(web.CodeForbidden, "Admins cannot apply to join an Idea."), auditReasonRole)
	}
	return markDenied(s.Gate.Precheck(ctx, caller), auditReasonState)
}

// checkEmpNo: exactly six ASCII digits, nothing is trimmed (PRD 6.3).
func checkEmpNo(s string) text.Problem {
	if text.Trim(s) == "" {
		return text.Required
	}
	if len(s) != empNoLen {
		return text.InvalidFormat
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return text.InvalidFormat
		}
	}
	return text.None
}

// validate cleans the body and reports every field problem in form order.
func validate(leaderEmpNo, ideaName string) (name string, err error) {
	var fields []web.FieldError
	if p := checkEmpNo(leaderEmpNo); p != text.None {
		fields = append(fields, web.FieldError{Field: fieldLeader, Code: web.ErrorCode(p)})
	}
	name, p := text.CheckName(ideaName, maxIdeaName)
	if p != text.None {
		fields = append(fields, web.FieldError{Field: fieldIdeaNam, Code: web.ErrorCode(p)})
	}
	if len(fields) == 0 {
		return name, nil
	}
	code := web.CodeValidationFailed
	if fields[0].Code == web.CodeInvalidNameChars {
		code = web.CodeInvalidNameChars
	}
	e := web.NewError(code, "the request has invalid fields")
	e.Fields = fields
	return "", e
}

// Apply runs POST /join-requests after AuthorizeApply (done by the handler
// before the body is read). Order (PRD 6.1, 6.7): body 422 → Idea lookup, 404
// → CANNOT_JOIN_OWN_IDEA 422 → transaction: AssertOpenInTx, INSERT PENDING
// (the primary key is the ALREADY_MEMBER / JOIN_REQUEST_EXISTS guard),
// JOIN_REQUEST_CREATE. A full team may still be applied to. An unknown
// transaction outcome answers 503.
func (s *Service) Apply(ctx context.Context, caller security.Principal, leaderEmpNo, ideaName string, req Request) (JoinRequest, error) {
	name, err := validate(leaderEmpNo, ideaName)
	if err != nil {
		return JoinRequest{}, err
	}
	got, err := s.Ideas.FindByLeader(ctx, leaderEmpNo, name)
	if err != nil {
		return JoinRequest{}, markDenied(err, auditReasonHidden)
	}
	if got.LeaderID == caller.UserID {
		return JoinRequest{}, web.NewError(web.CodeCannotJoinOwnIdea, "You cannot join your own Idea.")
	}

	var at time.Time
	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return markDenied(err, auditReasonState)
		}
		var err error
		at, err = store.InsertPending(ctx, tx, got.ID, caller.UserID)
		switch {
		case err == nil:
		case db.IsForeignKeyViolation(err):
			// The Idea was deleted after the lookup.
			return markDenied(web.NewError(web.CodeNotFound, ""), auditReasonHidden)
		default:
			if _, dup := db.DuplicateKey(err); !dup {
				return err
			}
			return s.existing(ctx, tx, got.ID, caller.UserID, err)
		}
		after, err := json.Marshal(map[string]any{"status": store.Pending})
		if err != nil {
			return fmt.Errorf("marshal JOIN_REQUEST_CREATE after: %w", err)
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.JoinRequestCreate, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: got.ID,
			Method: req.Method, Route: req.Route, Status: http.StatusCreated,
			IP: req.IP, UserAgent: req.UserAgent,
			After: after,
		})
	})
	switch out {
	case db.Unknown:
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check your join requests before retrying.")
		e.Cause = err
		return JoinRequest{}, e
	case db.Committed, db.RolledBack:
	}
	if err != nil {
		return JoinRequest{}, err
	}
	return JoinRequest{
		IdeaID: got.ID, IdeaName: got.Name, Category: got.Category, Org: got.Org, Direction: got.Direction,
		LeaderEmpNo: got.LeaderEmpNo, LeaderName: got.LeaderName, RequestedAt: at,
	}, nil
}

// existing maps the pair's current row to the 409 it deserves.
func (s *Service) existing(ctx context.Context, tx *sql.Tx, ideaID, userID uuid.UUID, cause error) error {
	st, err := store.Status(ctx, tx, ideaID, userID)
	if err != nil {
		return err
	}
	var e *web.Error
	if st == store.Accepted {
		e = web.NewError(web.CodeAlreadyMember, "You are already a Member of this Idea.")
	} else {
		e = web.NewError(web.CodeJoinRequestExists, "You already have a pending request for this Idea.")
	}
	e.Cause = cause
	return e
}
