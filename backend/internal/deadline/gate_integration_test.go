//go:build integration

package deadline_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// TestGateAssertOpenInTx runs the authoritative check inside a real write
// transaction for NOT_OPEN, OPEN and CLOSED, for a User and an Admin.
func TestGateAssertOpenInTx(t *testing.T) {
	ctx := context.Background()
	sdb := dbtest.New(t)
	g := deadline.Gate{Store: deadline.SQLStore{DB: sdb}}
	user := security.Principal{Role: security.RoleUser}
	admin := security.Principal{Role: security.RoleAdmin}

	assert := func(actor security.Principal) error {
		_, err := db.InTx(ctx, sdb, func(ctx context.Context, tx *sql.Tx) error {
			return g.AssertOpenInTx(ctx, tx, actor)
		})
		return err
	}
	set := func(t *testing.T, expr string) {
		t.Helper()
		if _, err := sdb.ExecContext(ctx, "UPDATE contest_setting SET deadline = "+expr+" WHERE id = 1"); err != nil {
			t.Fatal(err)
		}
	}
	code := func(err error) web.ErrorCode {
		var e *web.Error
		if errors.As(err, &e) {
			return e.Code
		}
		return ""
	}

	if got := code(assert(user)); got != web.CodeSubmissionNotOpen {
		t.Fatalf("NOT_OPEN user: %q", got)
	}
	if err := assert(admin); err != nil {
		t.Fatalf("NOT_OPEN admin: %v", err)
	}
	set(t, "NOW() + INTERVAL 1 HOUR")
	if err := assert(user); err != nil {
		t.Fatalf("OPEN user: %v", err)
	}
	set(t, "NOW() - INTERVAL 1 HOUR")
	if got := code(assert(user)); got != web.CodeSubmissionClosed {
		t.Fatalf("CLOSED user: %q", got)
	}
	if err := assert(admin); err != nil {
		t.Fatalf("CLOSED admin: %v", err)
	}
	if got := code(g.Precheck(ctx, user)); got != web.CodeSubmissionClosed {
		t.Fatalf("CLOSED precheck: %q", got)
	}
}
