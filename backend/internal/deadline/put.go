package deadline

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Year bounds of an accepted deadline, checked on the UTC instant (PRD 6.9).
const (
	minYear = 2000
	maxYear = 2099
)

// maxJSONBody is the JSON body limit (error-codes.md §2: 16 KiB → 413).
const maxJSONBody = 16 << 10

// Setter writes the deadline. It owns the transaction and the audit row.
type Setter struct {
	DB    *sql.DB
	Audit *audit.Service
}

// ParseDeadline parses a client value: RFC 3339 / ISO 8601 with a numeric
// offset or Z (fractions allowed, then truncated to the second, UTC). A value
// that does not parse, including one without an offset, is INVALID_FORMAT; a
// UTC year outside 2000–2099 is OUT_OF_RANGE. Both are 422 on field "deadline".
func ParseDeadline(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fieldErr(web.CodeInvalidFormat, "ISO 8601 date-time with offset or Z")
	}
	t = t.UTC().Truncate(time.Second)
	if y := t.Year(); y < minYear || y > maxYear {
		return time.Time{}, fieldErr(web.CodeOutOfRange, "year 2000-2099")
	}
	return t, nil
}

func fieldErr(code web.ErrorCode, expected string) *web.Error {
	e := web.NewError(web.CodeValidationFailed, "invalid deadline")
	e.Fields = []web.FieldError{{Field: "deadline", Code: code, Expected: expected}}
	return e
}

// docs renders the before/after audit documents. A nil deadline is JSON null.
func docs(before, after *time.Time) (b, a json.RawMessage, err error) {
	one := func(t *time.Time) (json.RawMessage, error) {
		var v *string
		if t != nil {
			s := web.FormatTime(*t)
			v = &s
		}
		return json.Marshal(map[string]*string{"deadline": v})
	}
	if b, err = one(before); err != nil {
		return nil, nil, fmt.Errorf("marshal DEADLINE_UPDATE before: %w", err)
	}
	if a, err = one(after); err != nil {
		return nil, nil, fmt.Errorf("marshal DEADLINE_UPDATE after: %w", err)
	}
	return b, a, nil
}

// Set replaces the deadline and writes DEADLINE_UPDATE (before and after) in
// the same transaction, then returns the new snapshot. The row is locked so
// two Admins serialise and each audit row's "before" is the true predecessor.
// An unknown commit outcome answers 503.
func (s Setter) Set(ctx context.Context, next time.Time, req audit.Event) (Snapshot, error) {
	var snap Snapshot
	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		var cur sql.NullTime
		if err := tx.QueryRowContext(ctx, `SELECT deadline FROM contest_setting WHERE id = 1 FOR UPDATE`).Scan(&cur); err != nil {
			return fmt.Errorf("lock contest_setting: %w", err)
		}
		var before *time.Time
		if cur.Valid {
			before = &cur.Time
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE contest_setting SET deadline = ?, updated_at = CURRENT_TIMESTAMP(6) WHERE id = 1`, next); err != nil {
			return fmt.Errorf("update contest_setting: %w", err)
		}
		b, a, err := docs(before, &next)
		if err != nil {
			return err
		}
		req.Action, req.Outcome = audit.DeadlineUpdate, audit.Success
		req.TargetType = audit.TargetDeadline
		req.Status = http.StatusOK
		req.Before, req.After = b, a
		if err := s.Audit.Record(ctx, tx, req); err != nil {
			return err
		}
		snap, err = TxStore{Tx: tx}.Snapshot(ctx)
		return err
	})
	switch out {
	case db.Committed:
		return snap, nil
	case db.Unknown:
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; read the deadline before retrying.")
		e.Cause = err
		return Snapshot{}, e
	case db.RolledBack:
	}
	return Snapshot{}, err
}

type putBody struct {
	Deadline *string `json:"deadline"`
}

// PutDeadline serves PUT /admin/deadline. 401 and CSRF are checked by the
// middleware; the role check runs before the body is read (PRD 6.1).
func (h Handler) PutDeadline(w http.ResponseWriter, r *http.Request) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	if !p.IsAdmin() {
		e := web.NewError(web.CodeForbidden, "Only an Admin can set the deadline.")
		e.AuditAction, e.AuditReason = string(audit.DeadlineUpdate), "role"
		e.AuditTargetType = string(audit.TargetDeadline)
		h.fail(w, r, e)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var b putBody
	if err := web.DecodeJSON(r, &b); err != nil {
		h.fail(w, r, err)
		return
	}
	next, err := ParseDeadline(*b.Deadline)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	snap, err := h.Setter.Set(r.Context(), next, audit.Event{
		Method: r.Method, Route: web.RoutePattern(r), IP: web.ClientIP(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, render(snap))
}

func (h Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	if h.Fail != nil {
		h.Fail(w, r, err)
		return
	}
	var e *web.Error
	if !errors.As(err, &e) {
		e = web.NewError(web.CodeInternalError, "")
		e.Cause = err
	}
	web.WriteProblem(w, r, e)
}
