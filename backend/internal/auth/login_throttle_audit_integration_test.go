//go:build integration

package auth_test

import (
	"strconv"
	"testing"
)

func (e loginEnv) throttledRows(t *testing.T) int {
	t.Helper()
	n := 0
	for _, r := range e.audits(t) {
		if r.Outcome == "THROTTLED" {
			n++
		}
	}
	return n
}

// A locked key answers 429 every time but is audited once per lock.
func TestLoginKeyLockAuditedOncePerWindow(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)
	for range 5 {
		e.fail(t, "123456")
	}
	for i := range 4 {
		if rec := e.login(t, creds("123456", password)); rec.Code != 429 {
			t.Fatalf("attempt %d: %d", i, rec.Code)
		}
	}
	if n := e.throttledRows(t); n != 1 {
		t.Fatalf("THROTTLED rows in one lock = %d, want 1", n)
	}
	// Another key is its own window.
	for range 5 {
		e.fail(t, "654321")
	}
	e.login(t, creds("654321", wrongPassword))
	if n := e.throttledRows(t); n != 2 {
		t.Fatalf("THROTTLED rows for two keys = %d, want 2", n)
	}
	// After the lock expires and the key locks again, a new window is audited.
	e.ageLock(t, "123456", 16)
	if _, err := e.db.Exec(`UPDATE login_throttle_audit SET audited_until = audited_until - INTERVAL 16 MINUTE`); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		e.fail(t, "123456")
	}
	e.login(t, creds("123456", wrongPassword))
	e.login(t, creds("123456", wrongPassword))
	if n := e.throttledRows(t); n != 3 {
		t.Fatalf("THROTTLED rows after second lock = %d, want 3", n)
	}
}

// The IP limit is audited once per window too, and a different key from the
// same address does not add a row.
func TestLoginIPLimitAuditedOncePerWindow(t *testing.T) {
	e := newLockEnv(t)
	e.seed(t, "192.0.2.9", 30, 0)
	for _, k := range []string{"111111", "222222", "333333"} {
		if rec := e.loginFrom(t, "192.0.2.9", k, wrongPassword, nil); rec.Code != 429 {
			t.Fatalf("%s: %d", k, rec.Code)
		}
	}
	if n := e.throttledRows(t); n != 1 {
		t.Fatalf("THROTTLED rows = %d, want 1", n)
	}
}

// Both limits at once: the larger Retry-After, and a new window on either side is audited.
func TestLoginBothLimitsLargerRetryAfter(t *testing.T) {
	e := newLockEnv(t)
	for range 5 {
		e.fail(t, "123456")
	}
	e.seed(t, "192.0.2.9", 30, 10*60) // IP frees in ~5 min, key lock holds ~15 min
	rec := e.login(t, creds("123456", wrongPassword))
	if got, _ := strconv.Atoi(rec.Header().Get("Retry-After")); rec.Code != 429 || got < 890 || got > 900 {
		t.Fatalf("%d Retry-After=%q, want ~900", rec.Code, rec.Header().Get("Retry-After"))
	}
	if n := e.throttledRows(t); n != 1 {
		t.Fatalf("THROTTLED rows = %d, want 1", n)
	}
}
