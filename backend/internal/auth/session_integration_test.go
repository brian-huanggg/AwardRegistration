//go:build integration

package auth_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

const bcryptHash = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234" // shape only; satisfies ck_app_user_hash

func addUser(t *testing.T, db *sql.DB, empNo, role string, enabled bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var email any = empNo + "@example.com"
	if role == "ADMIN" {
		email = nil
	}
	if _, err := db.Exec(`INSERT INTO app_user (id, emp_no, name, email, password_hash, role, enabled) VALUES (?,?,?,?,?,?,?)`,
		id.String(), empNo, "N "+empNo, email, bcryptHash, role, enabled); err != nil {
		t.Fatal(err)
	}
	return id
}

// age moves a session's timestamps into the past (DB time is the only clock,
// so tests change the rows rather than the clock).
func age(t *testing.T, db *sql.DB, id string, created, lastAccess time.Duration) {
	t.Helper()
	h, _ := auth.Binding(id)
	if _, err := db.Exec(`UPDATE session SET created_at = NOW(6) - INTERVAL ? SECOND, last_access_at = NOW(6) - INTERVAL ? SECOND WHERE id_hash = ?`,
		int(created.Seconds()), int(lastAccess.Seconds()), h); err != nil {
		t.Fatal(err)
	}
}

func lastAccess(t *testing.T, db *sql.DB, id string) time.Time {
	t.Helper()
	h, _ := auth.Binding(id)
	var ts time.Time
	if err := db.QueryRow(`SELECT last_access_at FROM session WHERE id_hash = ?`, h).Scan(&ts); err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestSessions(t *testing.T) {
	db := dbtest.New(t)
	ctx := context.Background()
	s := auth.New(db)
	user := addUser(t, db, "100001", "USER", true)

	create := func(t *testing.T, u uuid.UUID) string {
		t.Helper()
		id, err := s.Create(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	valid := func(t *testing.T, id string) bool {
		t.Helper()
		_, ok, err := s.Validate(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	t.Run("create stores only the SHA-256 and returns a 32-byte ID", func(t *testing.T) {
		id := create(t, user)
		raw, err := base64.RawURLEncoding.DecodeString(id)
		if err != nil || len(raw) != 32 {
			t.Fatalf("id %q: %d bytes, %v", id, len(raw), err)
		}
		sum := sha256.Sum256(raw)
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM session WHERE id_hash = ?`, sum[:]).Scan(&n); err != nil || n != 1 {
			t.Fatalf("hash row count = %d, %v", n, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM session WHERE id_hash = ?`, raw).Scan(&n); err != nil || n != 0 {
			t.Fatalf("raw ID stored: %d, %v", n, err)
		}
	})

	t.Run("every login gets a new ID", func(t *testing.T) {
		if a, b := create(t, user), create(t, user); a == b {
			t.Fatal("same ID twice")
		}
	})

	t.Run("validate returns the principal with the role from app_user", func(t *testing.T) {
		admin := addUser(t, db, "200001", "ADMIN", true)
		p, ok, err := s.Validate(ctx, create(t, admin))
		if err != nil || !ok {
			t.Fatalf("validate: %v %v", ok, err)
		}
		want := security.Principal{UserID: admin, EmpNo: "200001", Role: security.RoleAdmin}
		if p != want {
			t.Fatalf("principal = %+v, want %+v", p, want)
		}
	})

	t.Run("unknown and malformed IDs are not valid", func(t *testing.T) {
		unknown := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
		for _, id := range []string{unknown, "", "garbage"} {
			if valid(t, id) {
				t.Errorf("%q accepted", id)
			}
		}
	})

	t.Run("matched rows count: a valid session that needs no refresh still validates", func(t *testing.T) {
		id := create(t, user)
		before := lastAccess(t, db, id)
		for i := 0; i < 2; i++ {
			if !valid(t, id) {
				t.Fatalf("validate #%d failed although the row matched", i+1)
			}
		}
		if after := lastAccess(t, db, id); !after.Equal(before) {
			t.Fatalf("last_access_at moved inside the 60 s window: %v -> %v", before, after)
		}
	})

	t.Run("refresh happens only when last access is older than 60 s", func(t *testing.T) {
		id := create(t, user)
		age(t, db, id, 0, 59*time.Second)
		before := lastAccess(t, db, id)
		if !valid(t, id) || !lastAccess(t, db, id).Equal(before) {
			t.Fatal("59 s old: must validate without writing")
		}
		age(t, db, id, 0, 61*time.Second)
		before = lastAccess(t, db, id)
		if !valid(t, id) || !lastAccess(t, db, id).After(before) {
			t.Fatal("61 s old: must validate and refresh")
		}
	})

	t.Run("idle timeout is 30 minutes", func(t *testing.T) {
		id := create(t, user)
		age(t, db, id, 0, 29*time.Minute+50*time.Second)
		if !valid(t, id) {
			t.Fatal("29m50s idle must still be valid")
		}
		age(t, db, id, 0, 30*time.Minute+5*time.Second)
		if valid(t, id) {
			t.Fatal("30m05s idle must be expired")
		}
	})

	t.Run("absolute timeout is 8 hours, even for an active session", func(t *testing.T) {
		id := create(t, user)
		age(t, db, id, 7*time.Hour+59*time.Minute, 0)
		if !valid(t, id) {
			t.Fatal("7h59m old must still be valid")
		}
		age(t, db, id, 8*time.Hour+time.Minute, 0)
		if valid(t, id) {
			t.Fatal("8h01m old must be expired although just used")
		}
	})

	t.Run("disabled user is rejected on the next request", func(t *testing.T) {
		u := addUser(t, db, "100002", "USER", true)
		id := create(t, u)
		if !valid(t, id) {
			t.Fatal("enabled user must validate")
		}
		if _, err := db.Exec(`UPDATE app_user SET enabled = FALSE WHERE id = ?`, u.String()); err != nil {
			t.Fatal(err)
		}
		if valid(t, id) {
			t.Fatal("disabled user must not validate")
		}
	})

	t.Run("revoke is idempotent and ends only that session", func(t *testing.T) {
		a, b := create(t, user), create(t, user)
		for i := 0; i < 2; i++ {
			if err := s.Revoke(ctx, a); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Revoke(ctx, "garbage"); err != nil {
			t.Fatal(err)
		}
		if valid(t, a) || !valid(t, b) {
			t.Fatal("revoke must end a and leave b")
		}
	})

	t.Run("revoke all ends every session of the user only", func(t *testing.T) {
		u, other := addUser(t, db, "100003", "USER", true), addUser(t, db, "100004", "USER", true)
		a, b, o := create(t, u), create(t, u), create(t, other)
		if err := s.RevokeAll(ctx, u); err != nil {
			t.Fatal(err)
		}
		if valid(t, a) || valid(t, b) || !valid(t, o) {
			t.Fatal("RevokeAll must end the user's sessions and no others")
		}
	})

	t.Run("purge removes expired sessions only", func(t *testing.T) {
		u := addUser(t, db, "100005", "USER", true)
		live, idle, old := create(t, u), create(t, u), create(t, u)
		age(t, db, idle, time.Hour, time.Hour)
		age(t, db, old, 9*time.Hour, 0)
		if err := (auth.Purger{DB: db, Log: slog.New(slog.DiscardHandler)}).Purge(ctx); err != nil {
			t.Fatal(err)
		}
		if !valid(t, live) {
			t.Fatal("live session purged")
		}
		for _, id := range []string{idle, old} {
			h, _ := auth.Binding(id)
			var c int
			if err := db.QueryRow(`SELECT COUNT(*) FROM session WHERE id_hash = ?`, h).Scan(&c); err != nil || c != 0 {
				t.Fatalf("expired row still present: %d, %v", c, err)
			}
		}
	})

	t.Run("deleting a user deletes the sessions", func(t *testing.T) {
		u := addUser(t, db, "100006", "USER", true)
		id := create(t, u)
		if _, err := db.Exec(`DELETE FROM app_user WHERE id = ?`, u.String()); err != nil {
			t.Fatal(err)
		}
		if valid(t, id) {
			t.Fatal("session survived its user")
		}
	})
}
