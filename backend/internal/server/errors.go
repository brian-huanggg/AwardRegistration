package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Rejecter writes track-B audit rows. *audit.Service implements it.
type Rejecter interface {
	RecordRejected(ctx context.Context, e audit.Event)
}

// ErrorAdapter is the one place that turns a handler's error into a response
// and, when the error asks for it, into a track-B audit row (Backend.md §7).
// No other code calls RecordRejected.
type ErrorAdapter struct {
	Audit Rejecter
	Log   *slog.Logger // nil = discard
}

// Write answers err as problem+json. When err wraps a *web.Error with an
// AuditReason, the rejected attempt is recorded first (best effort: the row's
// failure never changes the response). Errors without a reason are not audited,
// and neither is anything that is not a *web.Error (it answers 500).
func (a ErrorAdapter) Write(w http.ResponseWriter, r *http.Request, err error) {
	var e *web.Error
	if !errors.As(err, &e) {
		web.WriteError(w, r, err)
		return
	}
	if e.AuditReason != "" {
		a.record(r, e)
	}
	web.WriteProblem(w, r, e)
}

func (a ErrorAdapter) record(r *http.Request, e *web.Error) {
	log := a.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if a.Audit == nil || e.AuditAction == "" {
		// A programming error: the attempt cannot be recorded. Say so loudly,
		// but still answer the client.
		log.ErrorContext(r.Context(), "audit: rejected attempt without action or recorder",
			"code", string(e.Code), "reason", e.AuditReason)
		return
	}
	outcome := audit.Outcome(e.AuditOutcome)
	if outcome == "" {
		outcome = audit.Denied
	}
	detail := map[string]string{}
	maps.Copy(detail, e.AuditDetail)
	detail["reason"] = e.AuditReason // the reason always wins over a detail key
	after, _ := json.Marshal(detail) // map of strings cannot fail
	ev := audit.Event{
		Action:     audit.Action(e.AuditAction),
		Outcome:    outcome,
		TargetType: audit.TargetType(e.AuditTargetType),
		TargetID:   e.AuditTargetID,
		Method:     r.Method,
		Route:      web.RoutePattern(r),
		Status:     e.Status,
		ErrorCode:  string(e.Code),
		IP:         web.ClientIP(r),
		UserAgent:  r.UserAgent(),

		SubmittedEmpNo: e.AuditSubmittedEmpNo,
		After:          after,
	}
	a.Audit.RecordRejected(r.Context(), ev)
}
