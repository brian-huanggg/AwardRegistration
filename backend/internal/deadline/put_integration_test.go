//go:build integration

package deadline_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/test/apispec"
)

func TestPutDeadlineIntegration(t *testing.T) {
	db := dbtest.New(t)
	spec := apispec.Load(t)
	h := spec.Validate(t, server.NewPublic(slog.New(slog.DiscardHandler), server.Mount(server.API{Deadline: deadline.Handler{
		Store:  deadline.SQLStore{DB: db},
		Setter: deadline.Setter{DB: db, Audit: audit.New(db, nil)},
	}})))

	put := func(p security.Principal, ct, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/deadline", strings.NewReader(body))
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		req = req.WithContext(security.WithPrincipal(context.Background(), p))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	admin := security.Principal{EmpNo: "900001", Role: security.RoleAdmin}
	user := security.Principal{EmpNo: "100001", Role: security.RoleUser}
	audits := func(outcome string) int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='DEADLINE_UPDATE' AND outcome=?`, outcome).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	t.Run("null to value, then value to value", func(t *testing.T) {
		rec := put(admin, "application/json", `{"deadline":"2090-05-01T18:00:00.9+08:00"}`)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"deadline":"2090-05-01T10:00:00.000Z"`) ||
			!strings.Contains(rec.Body.String(), `"state":"OPEN"`) {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		if rec = put(admin, "application/json", `{"deadline":"2020-01-01T00:00:00Z"}`); rec.Code != 200 ||
			!strings.Contains(rec.Body.String(), `"state":"CLOSED"`) {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		rows, err := db.Query(`SELECT before_json, after_json FROM audit_log WHERE action='DEADLINE_UPDATE' AND outcome='SUCCESS' ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		want := [][2]string{
			{`{"deadline": null}`, `{"deadline": "2090-05-01T10:00:00.000Z"}`},
			{`{"deadline": "2090-05-01T10:00:00.000Z"}`, `{"deadline": "2020-01-01T00:00:00.000Z"}`},
		}
		for i := 0; rows.Next(); i++ {
			var b, a string
			if err := rows.Scan(&b, &a); err != nil {
				t.Fatal(err)
			}
			if i >= len(want) || strings.ReplaceAll(b, " ", "") != strings.ReplaceAll(want[i][0], " ", "") ||
				strings.ReplaceAll(a, " ", "") != strings.ReplaceAll(want[i][1], " ", "") {
				t.Errorf("row %d: before=%s after=%s", i, b, a)
			}
		}
	})

	before := audits("SUCCESS")
	for _, tc := range []struct {
		name, ct, body string
		p              security.Principal
		status         int
	}{
		{"null", "application/json", `{"deadline":null}`, admin, 400},
		{"missing", "application/json", `{}`, admin, 400},
		{"wrong type", "application/json", `{"deadline":5}`, admin, 400},
		{"not json", "text/plain", `x`, admin, 415},
		{"no offset", "application/json", `{"deadline":"2030-01-01T00:00:00"}`, admin, 422},
		{"year 1999", "application/json", `{"deadline":"1999-12-31T23:59:59Z"}`, admin, 422},
		{"year 2100", "application/json", `{"deadline":"2100-01-01T00:00:00Z"}`, admin, 422},
		{"user", "application/json", `{"deadline":"2030-01-01T00:00:00Z"}`, user, 403},
	} {
		if rec := put(tc.p, tc.ct, tc.body); rec.Code != tc.status {
			t.Errorf("%s: %d %s", tc.name, rec.Code, rec.Body)
		}
	}
	if got := audits("SUCCESS"); got != before {
		t.Errorf("rejected requests wrote %d SUCCESS rows", got-before)
	}
	var d *string
	if err := db.QueryRow(`SELECT DATE_FORMAT(deadline,'%Y-%m-%d') FROM contest_setting`).Scan(&d); err != nil || d == nil || *d != "2020-01-01" {
		t.Errorf("deadline changed by a rejected request: %v %v", d, err)
	}
}
