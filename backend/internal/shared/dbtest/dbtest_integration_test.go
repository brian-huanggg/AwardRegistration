//go:build integration

package dbtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/clock"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

func TestPinnedSession(t *testing.T) {
	db := dbtest.New(t)
	ctx := context.Background()

	t.Run("session time zone is UTC", func(t *testing.T) {
		var tz string
		if err := db.QueryRowContext(ctx, `SELECT @@session.time_zone`).Scan(&tz); err != nil {
			t.Fatal(err)
		}
		if tz != "+00:00" {
			t.Fatalf("session time_zone = %q", tz)
		}
	})

	t.Run("UPDATE reports matched rows, not changed rows", func(t *testing.T) {
		// Setting a column to the value it already has changes nothing but matches one row.
		res, err := db.ExecContext(ctx, `UPDATE contest_setting SET deadline = deadline WHERE id = 1`)
		if err != nil {
			t.Fatal(err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			t.Fatalf("RowsAffected = %d, want 1 (clientFoundRows)", n)
		}
	})

	t.Run("DbClock is database time in UTC with microseconds", func(t *testing.T) {
		before := time.Now().UTC().Add(-time.Minute)
		got, err := clock.DbClock{DB: db}.Now(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.Location() != time.UTC {
			t.Fatalf("location = %v", got.Location())
		}
		if got.Before(before) || got.After(time.Now().UTC().Add(time.Minute)) {
			t.Fatalf("clock %v is far from the host clock", got)
		}
	})
}
