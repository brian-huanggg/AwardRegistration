package auth

import (
	"context"
	"database/sql"
	"net/netip"
	"strings"
	"time"
	"unicode"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth/internal/store"
)

const (
	loginMaxFailures = 5
	loginWindow      = 15 * time.Minute
	loginLockFor     = 15 * time.Minute
	loginKeyMax      = 32
)

// DBThrottle is the Throttle over the login_lock table: 5 consecutive
// failures, each within 15 minutes of the previous one, lock the submitted
// employee-ID string for a fixed 15 minutes (PRD §6.5, OI-12). The 5th failure
// still answers 401; the lock shows from the 6th attempt. The counters live in
// MariaDB so all replicas share them, and every time is database time.
//
// The per-IP window (B1b-05) is not part of this type yet: ip is ignored.
type DBThrottle struct{ DB *sql.DB }

var lockPolicy = store.LockPolicy{
	MaxFailures:   loginMaxFailures,
	WindowSeconds: int(loginWindow / time.Second),
	LockSeconds:   int(loginLockFor / time.Second),
}

// LockKey normalises the submitted employee-ID string: control characters are
// removed and the rest is cut to 32 characters.
func LockKey(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	if r := []rune(s); len(r) > loginKeyMax {
		s = string(r[:loginKeyMax])
	}
	return s
}

// Check implements Throttle.
func (t DBThrottle) Check(ctx context.Context, key string, _ netip.Addr) (time.Duration, error) {
	return store.LockRemaining(ctx, t.DB, LockKey(key))
}

// Fail implements Throttle.
func (t DBThrottle) Fail(ctx context.Context, key string, _ netip.Addr) error {
	return store.RecordFailure(ctx, t.DB, LockKey(key), lockPolicy)
}

// Succeed implements Throttle.
func (t DBThrottle) Succeed(ctx context.Context, key string) error {
	return store.ClearFailures(ctx, t.DB, LockKey(key))
}
