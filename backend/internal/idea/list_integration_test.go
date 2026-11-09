//go:build integration

package idea_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

func TestListIdeasHandler(t *testing.T) {
	d := dbtest.New(t)
	a := addUser(t, d, "100001", "USER")
	b := addUser(t, d, "100002", "USER")
	admin := addUser(t, d, "900001", "ADMIN")

	// a owns 3 Ideas, b owns 1 (different org/category). created_at is set explicitly.
	mk := func(leader security.Principal, name, org, cat, at string) string {
		n := newIdea(leader.UserID, name)
		n.Org, n.Category = org, cat
		if err := insert(d, n); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Exec(`UPDATE idea SET created_at = ? WHERE id = ?`, at, n.ID.String()); err != nil {
			t.Fatal(err)
		}
		return n.ID.String()
	}
	a1 := mk(a, "A1", "BSID", "COOL_IDEA", "2026-03-01 00:00:00.000000")
	a2 := mk(a, "A2", "BSID", "COOL_IDEA", "2026-03-02 00:00:00.000000")
	a3 := mk(a, "A3", "AAID", "OUTSTANDING_IMPACT", "2026-03-03 00:00:00.000000")
	b1 := mk(b, "B1", "AAID", "COOL_IDEA", "2026-03-04 00:00:00.000000")

	aud := audit.New(d, nil)
	errs := server.ErrorAdapter{Audit: aud}
	h := idea.Handler{Service: &idea.Service{DB: d, Audit: aud}, ExpectedFilename: expectedBase, Fail: errs.Write}

	call := func(p security.Principal, query string) (int, map[string]any) {
		v, _ := url.ParseQuery(query)
		var params api.ListIdeasParams
		get := func(k string) *string {
			if !v.Has(k) {
				return nil
			}
			s := v.Get(k)
			return &s
		}
		geti := func(k string) *int {
			if !v.Has(k) {
				return nil
			}
			var n int
			_, _ = fmt.Sscan(v.Get(k), &n)
			return &n
		}
		params.Page, params.Size = geti("page"), geti("size")
		params.Org, params.Category, params.Direction = get("org"), get("category"), get("direction")
		params.LeaderEmpNo, params.CreatedFrom, params.CreatedTo = get("leaderEmpNo"), get("createdFrom"), get("createdTo")
		params.Q, params.Sort, params.Order = get("q"), get("sort"), get("order")
		req := httptest.NewRequest(http.MethodGet, "/ideas?"+query, nil)
		req = req.WithContext(security.WithPrincipal(context.Background(), p))
		rec := httptest.NewRecorder()
		h.ListIdeas(rec, req, params)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}
	ids := func(body map[string]any) (out []string) {
		for _, it := range body["items"].([]any) {
			out = append(out, it.(map[string]any)["id"].(string))
		}
		return out
	}
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// User sees only own Ideas, newest first; myRelation LEADER; envelope.
	code, body := call(a, "")
	if code != 200 || !eq(ids(body), []string{a3, a2, a1}) || body["total"].(float64) != 3 ||
		body["page"].(float64) != 1 || body["size"].(float64) != 20 {
		t.Fatalf("user list: %d %v", code, body)
	}
	if rel := body["items"].([]any)[0].(map[string]any)["myRelation"]; rel != "LEADER" {
		t.Errorf("myRelation = %v", rel)
	}
	// Admin sees all, myRelation null.
	code, body = call(admin, "")
	if code != 200 || !eq(ids(body), []string{b1, a3, a2, a1}) {
		t.Fatalf("admin list: %d %v", code, body)
	}
	if rel, ok := body["items"].([]any)[0].(map[string]any)["myRelation"]; !ok || rel != nil {
		t.Errorf("admin myRelation = %v", rel)
	}
	// A user with nothing gets [] not null.
	if _, body = call(addUser(t, d, "100003", "USER"), ""); body["items"] == nil || body["total"].(float64) != 0 {
		t.Errorf("empty: %v", body)
	}

	// Paging: size 2, page 2; beyond last page is 200 and empty with total intact.
	_, body = call(admin, "size=2&page=2")
	if !eq(ids(body), []string{a2, a1}) || body["total"].(float64) != 4 {
		t.Errorf("page 2: %v", body)
	}
	code, body = call(admin, "size=2&page=3")
	if code != 200 || len(ids(body)) != 0 || body["total"].(float64) != 4 {
		t.Errorf("page 3: %d %v", code, body)
	}

	// Filters.
	for q, want := range map[string][]string{
		"org=BSID":                      {a2, a1},
		"category=OUTSTANDING_IMPACT":   {a3},
		"leaderEmpNo=100002":            {b1},
		"org=AAID&category=COOL_IDEA":   {b1},
		"direction=TALENT_DEVELOPMENT":  {b1, a3, a2, a1},
		"direction=GREEN_MANUFACTURING": nil,
		// [from, to) is half-open: 03-02 is included, 03-04 (== to) is not.
		"createdFrom=2026-03-02T00:00:00Z&createdTo=2026-03-04T00:00:00Z": {a3, a2},
		// Offset is honoured: 08:00+08:00 is 00:00Z.
		"createdFrom=2026-03-04T08:00:00%2B08:00": {b1},
	} {
		if _, body = call(admin, q); !eq(ids(body), want) {
			t.Errorf("%s: got %v want %v", q, ids(body), want)
		}
	}
	// Filters never widen visibility.
	if _, body = call(a, "leaderEmpNo=100002"); len(ids(body)) != 0 {
		t.Errorf("user saw another Leader's Idea: %v", body)
	}

	// 422 with the right field code; top-level VALIDATION_FAILED.
	for q, field := range map[string]string{
		"size=101": "size", "page=0": "page", "org=XXXX": "org", "category=": "category",
		"createdFrom=2026-03-01": "createdFrom", "leaderEmpNo=1": "leaderEmpNo",
		"sort=id": "sort", "sort=": "sort", "order=up": "order", "order=ASC": "order",
	} {
		code, body = call(admin, q)
		if code != 422 || body["code"] != "VALIDATION_FAILED" ||
			body["errors"].([]any)[0].(map[string]any)["field"] != field {
			t.Errorf("%s: %d %v", q, code, body)
		}
	}
	if _, body = call(admin, "org=X"); body["errors"].([]any)[0].(map[string]any)["code"] != "INVALID_ENUM" {
		t.Errorf("org code: %v", body)
	}
}

func TestListIdeasSearchSort(t *testing.T) {
	d := dbtest.New(t)
	u := addUser(t, d, "100001", "USER")
	admin := addUser(t, d, "900001", "ADMIN")
	svc := &idea.Service{DB: d}

	mk := func(name, at string) string {
		n := newIdea(u.UserID, name)
		if err := insert(d, n); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Exec(`UPDATE idea SET created_at = ? WHERE id = ?`, at, n.ID.String()); err != nil {
			t.Fatal(err)
		}
		return n.ID.String()
	}
	pct := mk("100% green", "2026-03-01 00:00:00.000000")
	und := mk("a_b", "2026-03-02 00:00:00.000000")
	axb := mk("axb", "2026-03-03 00:00:00.000000")
	bang := mk("Hi!", "2026-03-04 00:00:00.000000")
	upper := mk("ZETA Solar", "2026-03-05 00:00:00.000000")
	zh := mk("綠能創意", "2026-03-06 00:00:00.000000")

	names := func(f idea.ListFilter) (out []string) {
		rows, _, err := svc.List(context.Background(), admin, f, 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			out = append(out, r.ID.String())
		}
		return out
	}
	same := func(got, want []string) bool { return fmt.Sprint(got) == fmt.Sprint(want) }

	for _, tc := range []struct {
		q    string
		want []string
	}{
		{"%", []string{pct}},      // not a wildcard
		{"_", []string{und}},      // not a wildcard: axb does not match
		{"a_b", []string{und}},    // axb does not match
		{"!", []string{bang}},     // the escape character itself is literal
		{"zeta", []string{upper}}, // case-insensitive
		{"SOLAR", []string{upper}},
		{"綠能", []string{zh}},
		{"nothing", nil},
		{"", []string{zh, upper, bang, axb, und, pct}}, // empty term is no filter
	} {
		if got := names(idea.ListFilter{Q: tc.q, Desc: true}); !same(got, tc.want) {
			t.Errorf("q=%q: got %v want %v", tc.q, got, tc.want)
		}
	}

	// created_at both ways; name by code point (digits < upper < lower < CJK).
	if got := names(idea.ListFilter{Desc: false}); !same(got, []string{pct, und, axb, bang, upper, zh}) {
		t.Errorf("createdAt asc: %v", got)
	}
	if got := names(idea.ListFilter{SortName: true}); !same(got, []string{pct, bang, upper, und, axb, zh}) {
		t.Errorf("name asc: %v", got)
	}
	if got := names(idea.ListFilter{SortName: true, Desc: true}); !same(got, []string{zh, axb, und, upper, bang, pct}) {
		t.Errorf("name desc: %v", got)
	}

	// Equal sort keys fall back to id (MariaDB UUID order, not text order), in the sort direction. Same created_at for all.
	if _, err := d.Exec(`UPDATE idea SET created_at = '2026-04-01 00:00:00.000000'`); err != nil {
		t.Fatal(err)
	}
	asc, desc := names(idea.ListFilter{}), names(idea.ListFilter{Desc: true})
	if len(asc) != 6 {
		t.Errorf("tie-break asc: %v", asc)
	}
	slices.Reverse(desc)
	if !same(asc, desc) {
		t.Errorf("tie-break desc is not the reverse of asc: %v vs %v", asc, desc)
	}

	// A User's search never reaches another Leader's Ideas.
	other := addUser(t, d, "100002", "USER")
	if rows, _, err := svc.List(context.Background(), other, idea.ListFilter{Q: "zeta"}, 10, 0); err != nil || len(rows) != 0 {
		t.Errorf("search widened visibility: %v %v", rows, err)
	}
}
