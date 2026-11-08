//go:build integration

package audit_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

type row struct {
	RequestID, ActorEmpNo, TargetType, TargetID, Method, Route, ErrorCode, IP, UserAgent, Submitted sql.NullString
	ActorRole, Action, Outcome                                                                      string
	Status                                                                                          sql.NullInt64
	Before, After                                                                                   sql.NullString
	PrevHash, Hash                                                                                  []byte
	OccurredAtSet                                                                                   bool
}

// floor is the highest audit_log.id before the current subtest; rows() sees only newer rows. The
// table is append-only, so tests never clear it (test/arch scans for that).
var floor int64

func rows(t *testing.T, d *sql.DB) []row {
	t.Helper()
	rs, err := d.Query(`SELECT request_id, actor_emp_no, target_type, target_id, method, route, error_code,
	  ip, user_agent, submitted_emp_no, actor_role, action, outcome, status, before_json, after_json,
	  prev_hash, hash, occurred_at IS NOT NULL FROM audit_log WHERE id > ? ORDER BY id`, floor)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var out []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.RequestID, &r.ActorEmpNo, &r.TargetType, &r.TargetID, &r.Method, &r.Route, &r.ErrorCode,
			&r.IP, &r.UserAgent, &r.Submitted, &r.ActorRole, &r.Action, &r.Outcome, &r.Status, &r.Before, &r.After,
			&r.PrevHash, &r.Hash, &r.OccurredAtSet); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func reset(t *testing.T, d *sql.DB) {
	t.Helper()
	if err := d.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM audit_log`).Scan(&floor); err != nil {
		t.Fatal(err)
	}
}

func TestAudit(t *testing.T) {
	d := dbtest.New(t)
	svc := audit.New(d, nil)
	reqID := uuid.NewString()

	// Request ID is stored by web.RequestID middleware; reproduce it through the real middleware.
	ctxWithReq := func(principal *security.Principal) context.Context {
		var got context.Context
		h := web.RequestID(handlerFunc(func(ctx context.Context) { got = ctx }))
		h.ServeHTTP(httptest.NewRecorder(), newReq(reqID))
		if principal != nil {
			return security.WithPrincipal(got, *principal)
		}
		return got
	}

	t.Run("track A is written in the caller's transaction and carries every field", func(t *testing.T) {
		reset(t, d)
		ctx := ctxWithReq(&security.Principal{UserID: uuid.New(), EmpNo: "E0001", Role: security.RoleUser})
		target := uuid.New()
		out, err := db.InTx(ctx, d, func(ctx context.Context, tx *sql.Tx) error {
			return svc.Record(ctx, tx, audit.Event{
				Action: audit.IdeaUpdate, Outcome: audit.Success,
				TargetType: audit.TargetIdea, TargetID: target,
				Method: "PATCH", Route: "/ideas/{id}", Status: 200,
				IP: netip.MustParseAddr("203.0.113.9"), UserAgent: "ua",
				Before: []byte(`{"name":"a"}`), After: []byte(`{"name":"b","password":"p"}`),
			})
		})
		if err != nil || out != db.Committed {
			t.Fatalf("InTx = %v, %v", out, err)
		}
		got := rows(t, d)
		if len(got) != 1 {
			t.Fatalf("rows = %d, want 1", len(got))
		}
		r := got[0]
		if r.RequestID.String != reqID || r.ActorEmpNo.String != "E0001" || r.ActorRole != "USER" ||
			r.Action != "IDEA_UPDATE" || r.Outcome != "SUCCESS" || r.TargetType.String != "IDEA" ||
			r.TargetID.String != target.String() || r.Method.String != "PATCH" || r.Route.String != "/ideas/{id}" ||
			r.Status.Int64 != 200 || r.UserAgent.String != "ua" || !r.OccurredAtSet {
			t.Errorf("row = %+v", r)
		}
		if r.IP.String != "::ffff:203.0.113.9" {
			t.Errorf("ip = %q (IPv4 is stored as INET6 mapped)", r.IP.String)
		}
		if r.Before.String != `{"name": "a"}` && r.Before.String != `{"name":"a"}` {
			t.Errorf("before = %q", r.Before.String)
		}
		if strings.Contains(r.After.String, "\"p\"") || !strings.Contains(r.After.String, "[REDACTED]") {
			t.Errorf("password not redacted: %q", r.After.String)
		}
		if r.PrevHash != nil || r.Hash != nil {
			t.Error("hash-chain columns must stay NULL")
		}
	})

	t.Run("track A rolls back with the transaction", func(t *testing.T) {
		reset(t, d)
		boom := errors.New("business failure")
		out, err := db.InTx(context.Background(), d, func(ctx context.Context, tx *sql.Tx) error {
			if err := svc.Record(ctx, tx, audit.Event{Action: audit.AuthLogout, Outcome: audit.Success}); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) || out != db.RolledBack {
			t.Fatalf("InTx = %v, %v", out, err)
		}
		if n := len(rows(t, d)); n != 0 {
			t.Errorf("rows = %d, want 0 (rolled back with the business write)", n)
		}
	})

	t.Run("track A failure fails the operation", func(t *testing.T) {
		reset(t, d)
		out, err := db.InTx(context.Background(), d, func(ctx context.Context, tx *sql.Tx) error {
			return svc.Record(ctx, tx, audit.Event{Action: "BOGUS", Outcome: audit.Success})
		})
		if err == nil || out != db.RolledBack {
			t.Errorf("InTx = %v, %v; want RolledBack and an error", out, err)
		}
	})

	t.Run("track B persists although the business transaction rolled back", func(t *testing.T) {
		reset(t, d)
		ctx := ctxWithReq(nil)
		_, err := db.InTx(ctx, d, func(ctx context.Context, tx *sql.Tx) error {
			svc.RecordRejected(ctx, audit.Event{
				Action: audit.IdeaDelete, Outcome: audit.Denied, TargetType: audit.TargetIdea,
				TargetID: uuid.New(), Method: "DELETE", Route: "/ideas/{id}", Status: 404, ErrorCode: "NOT_FOUND",
			})
			return errors.New("rejected")
		})
		if err == nil {
			t.Fatal("want error")
		}
		got := rows(t, d)
		if len(got) != 1 || got[0].Outcome != "DENIED" || got[0].ActorRole != "ANONYMOUS" ||
			got[0].ActorEmpNo.Valid || got[0].ErrorCode.String != "NOT_FOUND" || got[0].RequestID.String != reqID {
			t.Errorf("rows = %+v", got)
		}
	})

	t.Run("track B survives a cancelled request context", func(t *testing.T) {
		reset(t, d)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		svc.RecordRejected(ctx, audit.Event{Action: audit.AuthLogin, Outcome: audit.Failed, SubmittedEmpNo: "E0009"})
		if got := rows(t, d); len(got) != 1 || got[0].Submitted.String != "E0009" {
			t.Errorf("rows = %+v", got)
		}
	})

	t.Run("track B failure is counted, not returned", func(t *testing.T) {
		reset(t, d)
		before := svc.RejectedFailures()
		svc.RecordRejected(context.Background(), audit.Event{Action: "BOGUS", Outcome: audit.Denied})
		if svc.RejectedFailures() != before+1 {
			t.Error("failure not counted")
		}
		if n := len(rows(t, d)); n != 0 {
			t.Errorf("rows = %d", n)
		}
	})

	t.Run("hostile input is sanitized and truncated, never fatal", func(t *testing.T) {
		reset(t, d)
		svc.RecordRejected(context.Background(), audit.Event{
			Action: audit.AuthLogin, Outcome: audit.Failed,
			UserAgent:      strings.Repeat("創", 1000) + "\r\nforged",
			SubmittedEmpNo: "E1\x00\n" + strings.Repeat("9", 500),
			Route:          strings.Repeat("r", 1000), ErrorCode: strings.Repeat("c", 1000),
		})
		got := rows(t, d)
		if len(got) != 1 {
			t.Fatalf("rows = %d, want 1", len(got))
		}
		r := got[0]
		if n := len([]rune(r.UserAgent.String)); n != 256 {
			t.Errorf("user agent = %d code points, want 256", n)
		}
		if strings.ContainsAny(r.Submitted.String, "\x00\n") || len(r.Submitted.String) != 64 {
			t.Errorf("submitted = %q", r.Submitted.String)
		}
		if len(r.Route.String) != 200 || len(r.ErrorCode.String) != 64 {
			t.Errorf("route/errorCode lengths = %d/%d", len(r.Route.String), len(r.ErrorCode.String))
		}
	})

	t.Run("actor override wins over the context", func(t *testing.T) {
		reset(t, d)
		ctx := security.WithSystem(context.Background())
		_, err := db.InTx(ctx, d, func(ctx context.Context, tx *sql.Tx) error {
			return svc.Record(ctx, tx, audit.Event{Action: audit.AuthLogin, Outcome: audit.Success,
				ActorRole: security.ActorAdmin, ActorEmpNo: "A0001", TargetType: audit.TargetUser, TargetID: uuid.New()})
		})
		if err != nil {
			t.Fatal(err)
		}
		if r := rows(t, d)[0]; r.ActorRole != "ADMIN" || r.ActorEmpNo.String != "A0001" {
			t.Errorf("row = %+v", r)
		}
	})

	t.Run("system actor and deadline target without an ID", func(t *testing.T) {
		reset(t, d)
		ctx := security.WithSystem(context.Background())
		_, err := db.InTx(ctx, d, func(ctx context.Context, tx *sql.Tx) error {
			return svc.Record(ctx, tx, audit.Event{Action: audit.AdminSync, Outcome: audit.Success,
				TargetType: audit.TargetDeadline, After: []byte(`null`)})
		})
		if err != nil {
			t.Fatal(err)
		}
		if r := rows(t, d)[0]; r.ActorRole != "SYSTEM" || r.TargetID.Valid || r.After.Valid {
			t.Errorf("row = %+v", r)
		}
	})

	t.Run("database CHECKs reject values outside the sets", func(t *testing.T) {
		for _, stmt := range []string{
			`INSERT INTO audit_log (actor_role, action, outcome) VALUES ('ROOT', 'X', 'SUCCESS')`,
			`INSERT INTO audit_log (actor_role, action, outcome) VALUES ('USER', 'X', 'MAYBE')`,
			`INSERT INTO audit_log (actor_role, action, outcome, target_type) VALUES ('USER', 'X', 'SUCCESS', 'TEAM')`,
		} {
			if _, err := d.Exec(stmt); err == nil {
				t.Errorf("accepted: %s", stmt)
			}
		}
	})
}

type handlerFunc func(ctx context.Context)

func (f handlerFunc) ServeHTTP(_ http.ResponseWriter, r *http.Request) { f(r.Context()) }

func newReq(id string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(web.RequestIDHeader, id)
	return r
}
