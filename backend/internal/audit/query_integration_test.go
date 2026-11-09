//go:build integration

package audit_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

// seed inserts a row with an explicit occurred_at, so ties can be built.
func seed(t *testing.T, d *sql.DB, at time.Time, actor, action, outcome string, extra ...any) uint64 {
	t.Helper()
	res, err := d.Exec(`INSERT INTO audit_log (occurred_at, actor_emp_no, actor_role, action, outcome, target_type, target_id, ip)
	 VALUES (?, ?, 'USER', ?, ?, ?, ?, ?)`, at, actor, action, outcome,
		extra[0], extra[1], extra[2])
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return uint64(id)
}

func TestQueryPaging(t *testing.T) {
	d := dbtest.New(t)
	svc := audit.New(d, nil)
	ctx := context.Background()
	actor := "pg-" + uuid.NewString()[:8]
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	// 10 rows over 4 instants, with ties and microsecond differences.
	var ids []uint64
	for i := range 10 {
		at := base.Add(time.Duration(i/3) * time.Second).Add(time.Duration(i/3) * time.Microsecond)
		ids = append(ids, seed(t, d, at, actor, "IDEA_VIEW", "SUCCESS", nil, nil, nil))
	}
	slices.Reverse(ids) // newest first, by id within a tie

	t.Run("no duplicates, no gaps", func(t *testing.T) {
		var got []uint64
		var after *audit.Cursor
		for pages := 0; ; pages++ {
			if pages > 10 {
				t.Fatal("paging does not end")
			}
			res, err := svc.Query(ctx, audit.Query{ActorEmpNo: actor, Limit: 3, After: after}, audit.Event{})
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range res.Entries {
				got = append(got, e.ID)
			}
			if res.Next == nil {
				break
			}
			if len(res.Entries) != 3 {
				t.Fatalf("short page with a next cursor: %d", len(res.Entries))
			}
			c, ok := audit.DecodeCursor(res.Next.Encode())
			if !ok {
				t.Fatal("cursor does not decode")
			}
			after = &c
		}
		if !slices.Equal(got, ids) {
			t.Fatalf("got %v, want %v", got, ids)
		}
	})

	t.Run("exact page size has no next cursor", func(t *testing.T) {
		res, err := svc.Query(ctx, audit.Query{ActorEmpNo: actor, Limit: 10}, audit.Event{})
		if err != nil || len(res.Entries) != 10 || res.Next != nil {
			t.Fatalf("entries=%d next=%v err=%v", len(res.Entries), res.Next, err)
		}
	})

	t.Run("from inclusive, to exclusive", func(t *testing.T) {
		from, to := base.Add(time.Second+time.Microsecond), base.Add(2*time.Second+2*time.Microsecond)
		res, err := svc.Query(ctx, audit.Query{ActorEmpNo: actor, From: &from, To: &to, Limit: 100}, audit.Event{})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Entries) != 3 { // i = 3,4,5
			t.Fatalf("got %d rows, want 3", len(res.Entries))
		}
		to2 := from // empty window
		res, err = svc.Query(ctx, audit.Query{ActorEmpNo: actor, From: &from, To: &to2, Limit: 100}, audit.Event{})
		if err != nil || len(res.Entries) != 0 {
			t.Fatalf("empty window: %d rows, %v", len(res.Entries), err)
		}
	})
}

func TestQueryFilters(t *testing.T) {
	d := dbtest.New(t)
	svc := audit.New(d, nil)
	ctx := context.Background()
	actor := "fl-" + uuid.NewString()[:8]
	at := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
	idea, other := uuid.NewString(), uuid.NewString()
	v4, v6 := "10.1.2.3", "2001:db8::7"

	a := seed(t, d, at, actor, "IDEA_VIEW", "SUCCESS", "IDEA", idea, v4)
	b := seed(t, d, at.Add(time.Second), actor, "IDEA_UPDATE", "DENIED", "IDEA", other, v6)
	c := seed(t, d, at.Add(2*time.Second), actor, "AUTH_LOGIN", "FAILED", nil, nil, nil)

	ids := func(q audit.Query) []uint64 {
		t.Helper()
		q.ActorEmpNo, q.Limit = actor, 100
		res, err := svc.Query(ctx, q, audit.Event{})
		if err != nil {
			t.Fatal(err)
		}
		var out []uint64
		for _, e := range res.Entries {
			out = append(out, e.ID)
		}
		return out
	}
	for name, tc := range map[string]struct {
		q    audit.Query
		want []uint64
	}{
		"all":         {audit.Query{}, []uint64{c, b, a}},
		"one action":  {audit.Query{Actions: []audit.Action{audit.IdeaView}}, []uint64{a}},
		"two actions": {audit.Query{Actions: []audit.Action{audit.IdeaView, audit.AuthLogin}}, []uint64{c, a}},
		"outcome":     {audit.Query{Outcome: audit.Denied}, []uint64{b}},
		"target type": {audit.Query{TargetType: audit.TargetIdea}, []uint64{b, a}},
		"target id":   {audit.Query{TargetID: uuid.MustParse(idea)}, []uint64{a}},
		"ipv4":        {audit.Query{IP: netip.MustParseAddr(v4)}, []uint64{a}},
		"ipv6":        {audit.Query{IP: netip.MustParseAddr(v6)}, []uint64{b}},
		"combined":    {audit.Query{TargetType: audit.TargetIdea, Outcome: audit.Success}, []uint64{a}},
		"no match":    {audit.Query{Outcome: audit.Throttled}, nil},
	} {
		if got := ids(tc.q); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}

	t.Run("request ID", func(t *testing.T) {
		req := uuid.New()
		if _, err := d.Exec(`INSERT INTO audit_log (request_id, actor_emp_no, actor_role, action, outcome)
		 VALUES (?, ?, 'USER', 'IDEA_VIEW', 'SUCCESS')`, req.String(), actor); err != nil {
			t.Fatal(err)
		}
		if got := ids(audit.Query{RequestID: req}); len(got) != 1 {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("stored address reads back canonical", func(t *testing.T) {
		res, err := svc.Query(ctx, audit.Query{ActorEmpNo: actor, IP: netip.MustParseAddr(v4), Limit: 5}, audit.Event{})
		if err != nil || len(res.Entries) != 1 {
			t.Fatalf("%v %v", res.Entries, err)
		}
		if got := res.Entries[0].IP.String; got != "::ffff:10.1.2.3" {
			t.Fatalf("stored ip text = %q", got)
		}
	})
}

func TestQueryWritesAuditQuery(t *testing.T) {
	d := dbtest.New(t)
	svc := audit.New(d, nil)
	ctx := context.Background()
	actor := "aq-" + uuid.NewString()[:8]
	seed(t, d, time.Date(2032, 1, 1, 0, 0, 0, 0, time.UTC), actor, "IDEA_VIEW", "SUCCESS", nil, nil, nil)
	var before int64
	if err := d.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM audit_log`).Scan(&before); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Query(ctx, audit.Query{ActorEmpNo: actor, Outcome: audit.Success, Limit: 7},
		audit.Event{Method: "GET", Route: "/admin/audit-logs", ActorRole: "ADMIN", ActorEmpNo: "900001"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Entries {
		if e.Action == string(audit.AuditQuery) {
			t.Fatal("the AUDIT_QUERY row appears in the page it describes")
		}
	}
	var action, outcome, role, after string
	var status int
	err = d.QueryRow(`SELECT action, outcome, actor_role, status, after_json FROM audit_log WHERE id > ?`, before).
		Scan(&action, &outcome, &role, &status, &after)
	if err != nil {
		t.Fatal(err)
	}
	if action != "AUDIT_QUERY" || outcome != "SUCCESS" || role != "ADMIN" || status != 200 {
		t.Fatalf("row = %s %s %s %d", action, outcome, role, status)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(after), &got); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got["actorEmpNo"], got["outcome"], got["limit"]) != fmt.Sprint(actor, "SUCCESS", float64(7)) {
		t.Fatalf("filters used = %v", got)
	}
}
