package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Service writes audit rows.
type Service struct {
	db  *sql.DB
	log *slog.Logger

	rejectedFailures atomic.Uint64
}

// New returns a Service. log may be nil.
func New(database *sql.DB, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{db: database, log: log}
}

// RejectedFailures counts track-B rows that could not be written (the
// audit-write-failures metric of PRD §10 reads this).
func (s *Service) RejectedFailures() uint64 { return s.rejectedFailures.Load() }

// Record is track A: it inserts the row in the caller's transaction, so the row
// commits or rolls back with the business write. An error must make the caller
// roll back (fail-closed). Any outcome is allowed.
func (s *Service) Record(ctx context.Context, tx *sql.Tx, e Event) error {
	if tx == nil {
		return errors.New("audit: Record needs the caller's transaction")
	}
	return insert(ctx, tx, e)
}

// RecordRejected is track B: it writes a rejected attempt (DENIED, FAILED or
// THROTTLED) in its own transaction. It is best effort: a failure is logged and
// counted, never returned, and a client disconnect does not drop the row (the
// transaction runs on a context detached from ctx, as db.InTx does). Only the
// central error adapter calls it (Backend.md §7).
func (s *Service) RecordRejected(ctx context.Context, e Event) {
	if e.Outcome == Success {
		s.rejectedFailed(ctx, e, errors.New("audit: RecordRejected called with outcome SUCCESS"))
		return
	}
	_, err := db.InTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		return insert(ctx, tx, e)
	})
	if err != nil {
		s.rejectedFailed(ctx, e, err)
	}
}

func (s *Service) rejectedFailed(ctx context.Context, e Event, err error) {
	s.rejectedFailures.Add(1)
	s.log.ErrorContext(ctx, "audit: rejected attempt not recorded",
		"action", string(e.Action), "outcome", string(e.Outcome), "error", err)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

const insertSQL = `INSERT INTO audit_log
 (request_id, actor_emp_no, actor_role, action, outcome, target_type, target_id,
  method, route, status, error_code, ip, user_agent, submitted_emp_no, before_json, after_json)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// insert validates and sanitizes e, then writes it. occurred_at is the column
// default, i.e. database time.
func insert(ctx context.Context, x execer, e Event) error {
	if !e.Action.Valid() {
		return fmt.Errorf("audit: unknown action %q", e.Action)
	}
	if !e.Outcome.Valid() {
		return fmt.Errorf("audit: unknown outcome %q", e.Outcome)
	}
	if e.TargetType != "" && !e.TargetType.Valid() {
		return fmt.Errorf("audit: unknown target type %q", e.TargetType)
	}
	if e.TargetID != uuid.Nil && e.TargetType == "" {
		return errors.New("audit: target ID without target type")
	}

	role, empNo := e.ActorRole, e.ActorEmpNo
	if role == "" {
		role, empNo = security.ActorRoleFrom(ctx), security.ActorEmpNo(ctx)
	}
	switch role {
	case security.ActorUser, security.ActorAdmin, security.ActorAnonymous, security.ActorSystem:
	default:
		return fmt.Errorf("audit: unknown actor role %q", role)
	}

	before, err := cleanJSON(e.Before)
	if err != nil {
		return fmt.Errorf("before: %w", err)
	}
	after, err := cleanJSON(e.After)
	if err != nil {
		return fmt.Errorf("after: %w", err)
	}

	var reqID any
	if id, err := uuid.Parse(web.RequestIDFrom(ctx)); err == nil {
		reqID = id.String()
	}
	var targetID any
	if e.TargetID != uuid.Nil {
		targetID = e.TargetID.String()
	}
	var status any
	if e.Status > 0 {
		status = e.Status
	}
	var ip any
	if e.IP.IsValid() {
		ip = e.IP.WithZone("").String() // INET6 has no zone
	}

	_, err = x.ExecContext(ctx, insertSQL,
		reqID, nullStr(clean(empNo, maxEmpNo)), string(role), string(e.Action), string(e.Outcome),
		nullStr(string(e.TargetType)), targetID,
		nullStr(clean(e.Method, maxMethod)), nullStr(clean(e.Route, maxRoute)), status,
		nullStr(clean(e.ErrorCode, maxErrorCode)), ip, nullStr(clean(e.UserAgent, maxUserAgent)),
		nullStr(clean(e.SubmittedEmpNo, maxEmpNo)), nullBytes(before), nullBytes(after))
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullBytes(b []byte) any {
	if b == nil {
		return nil
	}
	return string(b) // JSON column: send text, not a binary blob
}
