package audit

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Page-size bounds of the Audit Log query (PRD 6.11).
const (
	DefaultLimit = 50
	MaxLimit     = 100
)

// maxCursorMicros keeps a decoded cursor inside year 9999, so time.UnixMicro
// cannot wrap.
const maxCursorMicros = 253402300799 * 1_000_000

// Cursor is the keyset position: the last row of the previous page.
type Cursor struct {
	OccurredAt time.Time // microsecond precision
	ID         uint64
}

// Encode returns the opaque form: base64url of "<unix microseconds>,<id>".
func (c Cursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(strconv.FormatInt(c.OccurredAt.UnixMicro(), 10) + "," + strconv.FormatUint(c.ID, 10)))
}

// DecodeCursor is the inverse of Encode. Anything that is not exactly what
// Encode produces is rejected, so one position has one spelling.
func DecodeCursor(s string) (Cursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, false
	}
	us, id, ok := strings.Cut(string(raw), ",")
	if !ok {
		return Cursor{}, false
	}
	micros, err := strconv.ParseInt(us, 10, 64)
	if err != nil || micros < 0 || micros > maxCursorMicros {
		return Cursor{}, false
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return Cursor{}, false
	}
	c := Cursor{OccurredAt: time.UnixMicro(micros).UTC(), ID: n}
	if c.Encode() != s { // rejects "+1", leading zeros and padding
		return Cursor{}, false
	}
	return c, true
}

// Query is a validated Audit Log query. Zero values mean "no filter".
type Query struct {
	From, To   *time.Time // [From, To)
	ActorEmpNo string
	Actions    []Action
	Outcome    Outcome
	TargetType TargetType
	TargetID   uuid.UUID
	RequestID  uuid.UUID
	IP         netip.Addr
	After      *Cursor
	Limit      int
}

// Entry is one audit row as the query returns it.
type Entry struct {
	ID             uint64
	OccurredAt     time.Time
	RequestID      sql.NullString
	ActorEmpNo     sql.NullString
	ActorRole      string
	Action         string
	Outcome        string
	TargetType     sql.NullString
	TargetID       sql.NullString
	Method         sql.NullString
	Route          sql.NullString
	Status         sql.NullInt64
	ErrorCode      sql.NullString
	IP             sql.NullString
	UserAgent      sql.NullString
	SubmittedEmpNo sql.NullString
	Before, After  []byte
}

// Result is one page; Next is nil on the last page.
type Result struct {
	Entries []Entry
	Next    *Cursor
}

const selectSQL = `SELECT id, occurred_at, request_id, actor_emp_no, actor_role, action, outcome,
 target_type, target_id, method, route, status, error_code, ip, user_agent, submitted_emp_no,
 before_json, after_json FROM audit_log`

// build returns the SQL of one page (limit+1 rows, to learn whether another
// page exists) and its arguments. The keyset condition is written out because
// row comparison is not used (Backend.md R3).
func (q Query) build() (string, []any) {
	var where []string
	var args []any
	add := func(cond string, a ...any) {
		where = append(where, cond)
		args = append(args, a...)
	}
	if q.From != nil {
		add("occurred_at >= ?", q.From.UTC())
	}
	if q.To != nil {
		add("occurred_at < ?", q.To.UTC())
	}
	if q.ActorEmpNo != "" {
		add("actor_emp_no = ?", q.ActorEmpNo)
	}
	if len(q.Actions) > 0 {
		marks := make([]string, len(q.Actions))
		as := make([]any, len(q.Actions))
		for i, a := range q.Actions {
			marks[i], as[i] = "?", string(a)
		}
		add("action IN ("+strings.Join(marks, ",")+")", as...)
	}
	if q.Outcome != "" {
		add("outcome = ?", string(q.Outcome))
	}
	if q.TargetType != "" {
		add("target_type = ?", string(q.TargetType))
	}
	if q.TargetID != uuid.Nil {
		add("target_id = ?", q.TargetID.String())
	}
	if q.RequestID != uuid.Nil {
		add("request_id = ?", q.RequestID.String())
	}
	if q.IP.IsValid() {
		add("ip = ?", q.IP.String())
	}
	if q.After != nil {
		at := q.After.OccurredAt.UTC()
		add("(occurred_at < ? OR (occurred_at = ? AND id < ?))", at, at, q.After.ID)
	}
	sqlText := selectSQL
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	sqlText += " ORDER BY occurred_at DESC, id DESC LIMIT ?"
	return sqlText, append(args, q.Limit+1)
}

// filtersUsed is the after_json of the AUDIT_QUERY row: the filters that were
// set, as the client sent them in normalized form. The cursor is not a filter.
func (q Query) filtersUsed() (json.RawMessage, error) {
	m := map[string]any{"limit": q.Limit}
	if q.From != nil {
		m["from"] = web.FormatTime(*q.From)
	}
	if q.To != nil {
		m["to"] = web.FormatTime(*q.To)
	}
	if q.ActorEmpNo != "" {
		m["actorEmpNo"] = q.ActorEmpNo
	}
	if len(q.Actions) > 0 {
		m["action"] = q.Actions
	}
	if q.Outcome != "" {
		m["outcome"] = q.Outcome
	}
	if q.TargetType != "" {
		m["targetType"] = q.TargetType
	}
	if q.TargetID != uuid.Nil {
		m["targetId"] = q.TargetID.String()
	}
	if q.RequestID != uuid.Nil {
		m["requestId"] = q.RequestID.String()
	}
	if q.IP.IsValid() {
		m["ip"] = q.IP.String()
	}
	if q.After != nil {
		m["paged"] = true
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal AUDIT_QUERY filters: %w", err)
	}
	return b, nil
}

// Query returns one page and writes the AUDIT_QUERY row in the same
// transaction (fail-closed: no row, no page). The row is written after the
// read, so it is never part of the page it describes. ev carries the request
// facts (method, route, IP, user agent); the caller is the actor. An unknown
// commit outcome answers 503.
func (s *Service) Query(ctx context.Context, q Query, ev Event) (Result, error) {
	var res Result
	out, err := db.InTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		text, args := q.build()
		rows, err := tx.QueryContext(ctx, text, args...)
		if err != nil {
			return fmt.Errorf("audit: query: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var e Entry
			if err := rows.Scan(&e.ID, &e.OccurredAt, &e.RequestID, &e.ActorEmpNo, &e.ActorRole, &e.Action,
				&e.Outcome, &e.TargetType, &e.TargetID, &e.Method, &e.Route, &e.Status, &e.ErrorCode, &e.IP,
				&e.UserAgent, &e.SubmittedEmpNo, &e.Before, &e.After); err != nil {
				return fmt.Errorf("audit: scan: %w", err)
			}
			res.Entries = append(res.Entries, e)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("audit: query rows: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("audit: close rows: %w", err)
		}
		if len(res.Entries) > q.Limit {
			res.Entries = res.Entries[:q.Limit]
			last := res.Entries[q.Limit-1]
			res.Next = &Cursor{OccurredAt: last.OccurredAt, ID: last.ID}
		}
		after, err := q.filtersUsed()
		if err != nil {
			return err
		}
		ev.Action, ev.Outcome, ev.Status, ev.After = AuditQuery, Success, http.StatusOK, after
		return s.Record(ctx, tx, ev)
	})
	switch out {
	case db.Committed:
		return res, nil
	case db.Unknown:
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; retry the query.")
		e.Cause = err
		return Result{}, e
	case db.RolledBack:
	}
	return Result{}, err
}
