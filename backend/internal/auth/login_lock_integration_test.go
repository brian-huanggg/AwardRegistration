//go:build integration

package auth_test

import (
	"database/sql"
	"log/slog"
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

const wrongPassword = "Wrong-Horse-9"

func newLockEnv(t *testing.T) loginEnv {
	t.Helper()
	return newLoginEnvWith(t, slog.New(slog.DiscardHandler), func(db *sql.DB) auth.Throttle { return auth.DBThrottle{DB: db} })
}

func (e loginEnv) fail(t *testing.T, empNo string) {
	t.Helper()
	if rec := e.login(t, creds(empNo, wrongPassword)); rec.Code != 401 {
		t.Fatalf("failure attempt: %d %s", rec.Code, rec.Body)
	}
}

func (e loginEnv) lockRow(t *testing.T, key string) (count int, locked bool) {
	t.Helper()
	err := e.db.QueryRow(`SELECT fail_count, COALESCE(locked_until > NOW(6), 0) FROM login_lock WHERE lock_key = ?`, key).Scan(&count, &locked)
	if err == sql.ErrNoRows {
		return 0, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return count, locked
}

// ageLock moves the key's lock state back by d minutes (time passing, DB clock).
func (e loginEnv) ageLock(t *testing.T, key string, minutes int) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE login_lock SET last_fail_at = last_fail_at - INTERVAL ? MINUTE,
		locked_until = locked_until - INTERVAL ? MINUTE WHERE lock_key = ?`, minutes, minutes, key); err != nil {
		t.Fatal(err)
	}
}

func TestLoginLockFifthFailureIs401SixthIs429(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	for range 5 {
		e.fail(t, "123456")
	}
	if n, locked := e.lockRow(t, "123456"); n != 5 || !locked {
		t.Fatalf("after 5th failure: count=%d locked=%v", n, locked)
	}
	rec := e.login(t, creds("123456", wrongPassword))
	if rec.Code != 429 || code(t, rec) != "LOGIN_THROTTLED" {
		t.Fatalf("6th attempt: %d %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Retry-After"); got != "900" && got != "899" {
		t.Errorf("Retry-After = %q, want ~900", got)
	}
}

func TestLoginLockCorrectPasswordDuringLockIs429AndDoesNotExtend(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	for range 5 {
		e.fail(t, "123456")
	}
	e.ageLock(t, "123456", 10) // 5 minutes left
	var before string
	if err := e.db.QueryRow(`SELECT locked_until FROM login_lock WHERE lock_key = '123456'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, pw := range []string{password, wrongPassword} {
		rec := e.login(t, creds("123456", pw))
		if rec.Code != 429 {
			t.Fatalf("during lock with %q: %d %s", pw, rec.Code, rec.Body)
		}
		if got := rec.Header().Get("Retry-After"); got != "300" && got != "299" {
			t.Errorf("Retry-After = %q, want ~300", got)
		}
	}
	var after string
	if err := e.db.QueryRow(`SELECT locked_until FROM login_lock WHERE lock_key = '123456'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("lock extended: %s → %s", before, after)
	}
	if e.sessions(t) != 0 {
		t.Error("session opened during lock")
	}
}

func TestLoginLockExpiresAfterFifteenMinutes(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	for range 5 {
		e.fail(t, "123456")
	}
	e.ageLock(t, "123456", 16)
	if rec := e.login(t, creds("123456", password)); rec.Code != 200 {
		t.Fatalf("after lock expiry: %d %s", rec.Code, rec.Body)
	}
	if n, _ := e.lockRow(t, "123456"); n != 0 {
		t.Errorf("counter not cleared by success: %d", n)
	}
}

func TestLoginLockCounterRestartsAfterExpiredLock(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	for range 5 {
		e.fail(t, "123456")
	}
	e.ageLock(t, "123456", 16)
	e.fail(t, "123456")
	if n, locked := e.lockRow(t, "123456"); n != 1 || locked {
		t.Errorf("count=%d locked=%v, want 1 / unlocked", n, locked)
	}
}

func TestLoginLockFailuresMustBeConsecutiveWithinWindow(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	for range 4 {
		e.fail(t, "123456")
	}
	e.ageLock(t, "123456", 16) // the 4 failures are stale
	e.fail(t, "123456")
	if n, locked := e.lockRow(t, "123456"); n != 1 || locked {
		t.Errorf("count=%d locked=%v, want 1 / unlocked", n, locked)
	}
	// Within the window the count keeps growing.
	e.ageLock(t, "123456", 14)
	for range 3 {
		e.fail(t, "123456")
	}
	if n, _ := e.lockRow(t, "123456"); n != 4 {
		t.Errorf("count = %d, want 4", n)
	}
}

func TestLoginLockSuccessClearsCounter(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	for range 4 {
		e.fail(t, "123456")
	}
	if rec := e.login(t, creds("123456", password)); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for range 4 {
		e.fail(t, "123456")
	}
	if rec := e.login(t, creds("123456", password)); rec.Code != 200 {
		t.Fatalf("counter not reset by success: %d %s", rec.Code, rec.Body)
	}
}

func TestLoginLockCountsUnknownAndMalformedKeys(t *testing.T) {
	e := newLockEnv(t)
	for _, key := range []string{"999999", "not-an-id"} {
		for range 5 {
			e.fail(t, key)
		}
		if rec := e.login(t, creds(key, wrongPassword)); rec.Code != 429 {
			t.Errorf("%q: 6th attempt = %d, want 429", key, rec.Code)
		}
	}
}

func TestLoginLockIsPerKeyAndDisabledAccountsCount(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	e.addUser(t, "654321", "USER", false)
	for range 5 {
		e.fail(t, "654321") // disabled: same 401, still counted
	}
	if rec := e.login(t, creds("654321", password)); rec.Code != 429 {
		t.Errorf("disabled key: %d", rec.Code)
	}
	if rec := e.login(t, creds("123456", password)); rec.Code != 200 {
		t.Errorf("other key affected: %d %s", rec.Code, rec.Body)
	}
}

func TestLoginLockKeyIsNormalised(t *testing.T) {
	e := newLockEnv(t)
	long := strings.Repeat("9", 40)
	for range 5 {
		e.fail(t, long)
	}
	// Same first 32 characters share the key.
	if rec := e.login(t, creds(strings.Repeat("9", 32)+"x", wrongPassword)); rec.Code != 429 {
		t.Errorf("cut key: %d", rec.Code)
	}
	if n, _ := e.lockRow(t, strings.Repeat("9", 32)); n != 5 {
		t.Errorf("count = %d, want 5", n)
	}
}
