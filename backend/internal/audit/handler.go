package audit

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// microLayout is the wire format of occurredAt: UTC, microsecond precision.
const microLayout = "2006-01-02T15:04:05.000000Z"

// maxEmpNoFilter matches the audit_log.actor_emp_no column.
const maxEmpNoFilter = maxEmpNo

// Handler serves GET /admin/audit-logs.
type Handler struct {
	Service *Service
	Fail    func(w http.ResponseWriter, r *http.Request, err error)
}

// ParseQuery validates every parameter and reports all failures at once, in
// parameter order, each parameter with its first error. No invalid value is
// ignored (PRD 6.2).
func ParseQuery(p api.ListAuditLogsParams) (Query, []web.FieldError) {
	q := Query{Limit: DefaultLimit}
	var errs []web.FieldError
	bad := func(field string, code web.ErrorCode) {
		errs = append(errs, web.FieldError{Field: field, Code: code})
	}
	instant := func(field string, v *string) *time.Time {
		if v == nil {
			return nil
		}
		t, err := time.Parse(time.RFC3339, *v) // offset or Z is required
		if err != nil {
			bad(field, web.CodeInvalidFormat)
			return nil
		}
		t = t.UTC().Truncate(time.Microsecond)
		return &t
	}
	q.From = instant("from", p.From)
	q.To = instant("to", p.To)
	if p.ActorEmpNo != nil {
		if n := len(*p.ActorEmpNo); n == 0 || n > maxEmpNoFilter {
			bad("actorEmpNo", web.CodeInvalidFormat)
		} else {
			q.ActorEmpNo = *p.ActorEmpNo
		}
	}
	if p.Action != nil {
		seen := map[Action]bool{}
		ok := true
		for _, v := range *p.Action {
			a := Action(v)
			if !a.Valid() {
				ok = false
				break
			}
			if !seen[a] {
				seen[a] = true
				q.Actions = append(q.Actions, a)
			}
		}
		if !ok {
			q.Actions = nil
			bad("action", web.CodeInvalidEnum)
		}
	}
	if p.Outcome != nil {
		if o := Outcome(*p.Outcome); o.Valid() {
			q.Outcome = o
		} else {
			bad("outcome", web.CodeInvalidEnum)
		}
	}
	if p.TargetType != nil {
		if t := TargetType(*p.TargetType); t.Valid() {
			q.TargetType = t
		} else {
			bad("targetType", web.CodeInvalidEnum)
		}
	}
	id := func(field string, v *string, dst *uuid.UUID) {
		if v == nil {
			return
		}
		u, err := uuid.Parse(*v)
		if err != nil || u == uuid.Nil || u.String() != *v {
			bad(field, web.CodeInvalidFormat)
			return
		}
		*dst = u
	}
	id("targetId", p.TargetId, &q.TargetID)
	id("requestId", p.RequestId, &q.RequestID)
	if p.Ip != nil {
		ip, err := netip.ParseAddr(*p.Ip)
		if err != nil || ip.Zone() != "" {
			bad("ip", web.CodeInvalidFormat)
		} else {
			q.IP = ip.Unmap()
		}
	}
	if p.Cursor != nil {
		if c, ok := DecodeCursor(*p.Cursor); ok {
			q.After = &c
		} else {
			bad("cursor", web.CodeInvalidFormat)
		}
	}
	if p.Limit != nil {
		n, err := strconv.Atoi(*p.Limit)
		switch {
		case err != nil:
			bad("limit", web.CodeInvalidFormat)
		case n < 1 || n > MaxLimit:
			bad("limit", web.CodeOutOfRange)
		default:
			q.Limit = n
		}
	}
	return q, errs
}

// entry is AuditLog (docs/API/response-bodies.md).
type entry struct {
	ID             uint64          `json:"id"`
	OccurredAt     string          `json:"occurredAt"`
	RequestID      *string         `json:"requestId"`
	ActorEmpNo     *string         `json:"actorEmpNo"`
	ActorRole      string          `json:"actorRole"`
	Action         string          `json:"action"`
	Outcome        string          `json:"outcome"`
	TargetType     *string         `json:"targetType"`
	TargetID       *string         `json:"targetId"`
	Method         *string         `json:"method"`
	Route          *string         `json:"route"`
	Status         *int64          `json:"status"`
	ErrorCode      *string         `json:"errorCode"`
	IP             *string         `json:"ip"`
	UserAgent      *string         `json:"userAgent"`
	SubmittedEmpNo *string         `json:"submittedEmpNo"`
	Before         json.RawMessage `json:"before"`
	After          json.RawMessage `json:"after"`
}

func str(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

// ip renders the stored address in canonical text: INET6 returns an IPv4
// address in its mapped form (::ffff:a.b.c.d), which becomes dotted.
func ip(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	a, err := netip.ParseAddr(n.String)
	if err != nil {
		return &n.String
	}
	s := a.Unmap().String()
	return &s
}

func raw(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil // marshals as null
	}
	return b
}

func toBody(e Entry) entry {
	var status *int64
	if e.Status.Valid {
		status = &e.Status.Int64
	}
	return entry{
		ID: e.ID, OccurredAt: e.OccurredAt.UTC().Format(microLayout), RequestID: str(e.RequestID),
		ActorEmpNo: str(e.ActorEmpNo), ActorRole: e.ActorRole, Action: e.Action, Outcome: e.Outcome,
		TargetType: str(e.TargetType), TargetID: str(e.TargetID), Method: str(e.Method), Route: str(e.Route),
		Status: status, ErrorCode: str(e.ErrorCode), IP: ip(e.IP), UserAgent: str(e.UserAgent),
		SubmittedEmpNo: str(e.SubmittedEmpNo), Before: raw(e.Before), After: raw(e.After),
	}
}

// ListAuditLogs serves GET /admin/audit-logs: {items, nextCursor}. 401 and the
// role check come before any parameter is looked at.
func (h Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request, params api.ListAuditLogsParams) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	if !p.IsAdmin() {
		e := web.NewError(web.CodeForbidden, "Only an Admin can query the Audit Log.")
		e.AuditAction, e.AuditReason = string(AuditQuery), "role"
		h.Fail(w, r, e)
		return
	}
	q, errs := ParseQuery(params)
	if len(errs) > 0 {
		e := web.NewError(web.CodeValidationFailed, "invalid query parameter")
		e.Fields = errs
		h.Fail(w, r, e)
		return
	}
	res, err := h.Service.Query(r.Context(), q, Event{
		Method: r.Method, Route: web.RoutePattern(r), IP: web.ClientIP(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	out := struct {
		Items      []entry `json:"items"`
		NextCursor *string `json:"nextCursor"`
	}{Items: make([]entry, 0, len(res.Entries))}
	for _, e := range res.Entries {
		out.Items = append(out.Items, toBody(e))
	}
	if res.Next != nil {
		c := res.Next.Encode()
		out.NextCursor = &c
	}
	web.WriteJSON(w, http.StatusOK, out)
}
