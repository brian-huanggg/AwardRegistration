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

	ipMaxFailures = 30
	ipWindow      = 15 * time.Minute
)

// DBThrottle is the Throttle over the login_lock table: 5 consecutive
// failures, each within 15 minutes of the previous one, lock the submitted
// employee-ID string for a fixed 15 minutes (PRD §6.5, OI-12). The 5th failure
// still answers 401; the lock shows from the 6th attempt. The counters live in
// MariaDB so all replicas share them, and every time is database time.
//
// It also applies the per-IP limit: 30 failures from one address inside a
// sliding 15-minute window answer 429 until the 30th newest failure leaves the
// window (login_attempt). A throttled attempt is neither counted for the key
// nor for the IP. An invalid (zero) ip skips the IP window.
type DBThrottle struct{ DB *sql.DB }

var lockPolicy = store.LockPolicy{
	MaxFailures:   loginMaxFailures,
	WindowSeconds: int(loginWindow / time.Second),
	LockSeconds:   int(loginLockFor / time.Second),
}

var ipPolicy = store.IPPolicy{MaxFailures: ipMaxFailures, WindowSeconds: int(ipWindow / time.Second)}

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

// Check implements Throttle. Both limits apply; the longer wait is returned.
func (t DBThrottle) Check(ctx context.Context, key string, ip netip.Addr) (time.Duration, error) {
	keyWait, err := store.LockRemaining(ctx, t.DB, LockKey(key))
	if err != nil {
		return 0, err
	}
	if !ip.IsValid() {
		return keyWait, nil
	}
	ipWait, err := store.IPRetryAfter(ctx, t.DB, ipString(ip), ipPolicy)
	if err != nil {
		return 0, err
	}
	return max(keyWait, ipWait), nil
}

// AuditOnce implements Throttle: the first throttled attempt of a lock or IP
// window claims the audit row; later ones in the same window do not. When both
// limits apply, the attempt is audited if either window is new.
func (t DBThrottle) AuditOnce(ctx context.Context, key string, ip netip.Addr) (bool, error) {
	var first bool
	if wait, err := store.LockRemaining(ctx, t.DB, LockKey(key)); err != nil {
		return false, err
	} else if wait > 0 {
		won, err := store.ClaimThrottleAudit(ctx, t.DB, "key", LockKey(key), wait)
		if err != nil {
			return false, err
		}
		first = first || won
	}
	if ip.IsValid() {
		wait, err := store.IPRetryAfter(ctx, t.DB, ipString(ip), ipPolicy)
		if err != nil {
			return false, err
		}
		if wait > 0 {
			won, err := store.ClaimThrottleAudit(ctx, t.DB, "ip", ipString(ip), wait)
			if err != nil {
				return false, err
			}
			first = first || won
		}
	}
	return first, nil
}

// Fail implements Throttle.
func (t DBThrottle) Fail(ctx context.Context, key string, ip netip.Addr) error {
	if err := store.RecordFailure(ctx, t.DB, LockKey(key), lockPolicy); err != nil {
		return err
	}
	if !ip.IsValid() {
		return nil
	}
	return store.RecordIPFailure(ctx, t.DB, ipString(ip))
}

// Succeed implements Throttle. The IP window is not cleared: its failures stay
// until they leave the window.
func (t DBThrottle) Succeed(ctx context.Context, key string) error {
	return store.ClearFailures(ctx, t.DB, LockKey(key))
}

func ipString(ip netip.Addr) string { return ip.Unmap().WithZone("").String() }
