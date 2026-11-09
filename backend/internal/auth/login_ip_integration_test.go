//go:build integration

package auth_test

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// loginFrom posts a login as peer ip (TCP address) with each empNo of its own
// so that the key lock never interferes with the IP window.
func (e loginEnv) loginFrom(t *testing.T, ip, empNo, pw string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	tok, ck := e.csrf(t, nil)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(creds(empNo, pw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", tok)
	req.RemoteAddr = ip + ":4000"
	req.AddCookie(ck)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return e.do(req)
}

func (e loginEnv) ipFailures(t *testing.T, ip string) (n int) {
	t.Helper()
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM login_attempt WHERE ip = ?`, ip).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// seed inserts n failures of ip, all age seconds old, without running logins
// (a real failure costs a bcrypt comparison, ~1 s under -race).
func (e loginEnv) seed(t *testing.T, ip string, n, ageSeconds int) {
	t.Helper()
	for range n {
		if _, err := e.db.Exec(`INSERT INTO login_attempt (ip, attempted_at) VALUES (?, NOW(6) - INTERVAL ? SECOND)`, ip, ageSeconds); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoginIP30thFailureIs401And31stIs429(t *testing.T) {
	e := newLockEnv(t)
	e.seed(t, "192.0.2.9", 29, 0)
	// The 30th failure is a real login: still 401, and it is recorded.
	if rec := e.loginFrom(t, "192.0.2.9", "100000", wrongPassword, nil); rec.Code != 401 {
		t.Fatalf("30th: %d %s", rec.Code, rec.Body)
	}
	if n := e.ipFailures(t, "192.0.2.9"); n != 30 {
		t.Fatalf("rows = %d", n)
	}
	rec := e.loginFrom(t, "192.0.2.9", "999999", wrongPassword, nil)
	if rec.Code != 429 || code(t, rec) != "LOGIN_THROTTLED" {
		t.Fatalf("31st: %d %s", rec.Code, rec.Body)
	}
	if got, _ := strconv.Atoi(rec.Header().Get("Retry-After")); got < 895 || got > 900 {
		t.Errorf("Retry-After = %q, want ~900", rec.Header().Get("Retry-After"))
	}
	// Throttled attempts are not counted, for the IP or the key.
	if n := e.ipFailures(t, "192.0.2.9"); n != 30 {
		t.Errorf("rows after throttled attempt = %d", n)
	}
	if n, _ := e.lockRow(t, "999999"); n != 0 {
		t.Errorf("key counted: %d", n)
	}
}

func TestLoginIPCorrectPasswordIsThrottledToo(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "ADMIN", true)
	e.seed(t, "192.0.2.9", 30, 0)
	if rec := e.loginFrom(t, "192.0.2.9", "123456", password, nil); rec.Code != 429 {
		t.Fatalf("admin from throttled IP: %d %s", rec.Code, rec.Body)
	}
	if rec := e.loginFrom(t, "192.0.2.10", "123456", password, nil); rec.Code != 200 {
		t.Fatalf("other IP: %d %s", rec.Code, rec.Body)
	}
}

func TestLoginIPWindowSlides(t *testing.T) {
	e := newLockEnv(t)
	e.seed(t, "192.0.2.9", 1, 14*60) // oldest: leaves the window in ~1 min
	e.seed(t, "192.0.2.9", 29, 60)
	rec := e.loginFrom(t, "192.0.2.9", "999999", wrongPassword, nil)
	if rec.Code != 429 {
		t.Fatalf("30 in window: %d", rec.Code)
	}
	if got, _ := strconv.Atoi(rec.Header().Get("Retry-After")); got < 50 || got > 61 {
		t.Errorf("Retry-After = %d, want ~60 (oldest leaves in 1 min)", got)
	}
	// Push the oldest out: 29 in the window, so the next attempt is allowed (and counted).
	if _, err := e.db.Exec(`UPDATE login_attempt SET attempted_at = attempted_at - INTERVAL 2 MINUTE ORDER BY attempted_at LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if rec := e.loginFrom(t, "192.0.2.9", "999999", wrongPassword, nil); rec.Code != 401 {
		t.Fatalf("29 in window: %d %s", rec.Code, rec.Body)
	}
	if rec := e.loginFrom(t, "192.0.2.9", "999998", wrongPassword, nil); rec.Code != 429 {
		t.Fatalf("back to 30: %d", rec.Code)
	}
}

// With more than 30 rows the wait is until the 30th newest leaves, not the oldest.
func TestLoginIPRetryAfterUsesThirtiethNewest(t *testing.T) {
	e := newLockEnv(t)
	e.seed(t, "192.0.2.9", 5, 14*60) // five old ones
	e.seed(t, "192.0.2.9", 30, 10)   // 30 recent: the 30th newest is 10 s old
	rec := e.loginFrom(t, "192.0.2.9", "999999", wrongPassword, nil)
	if got, _ := strconv.Atoi(rec.Header().Get("Retry-After")); rec.Code != 429 || got < 880 || got > 891 {
		t.Fatalf("%d Retry-After=%q, want ~890", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestLoginIPLimitIsPerAddress(t *testing.T) {
	e := newLockEnv(t)
	e.seed(t, "192.0.2.9", 30, 0)
	e.seed(t, "2001:db8::1", 5, 0)
	if rec := e.loginFrom(t, "2001:db8::1", "999999", wrongPassword, nil); rec.Code != 401 {
		t.Fatalf("other address: %d", rec.Code)
	}
}

func TestLoginIPUsesForwardedClientWhenTrusted(t *testing.T) {
	e := newLockEnv(t)
	e.h = web.TrustedProxies(1)(e.h)
	hdr := map[string]string{"X-Forwarded-For": "10.9.9.9, 198.51.100.1"}
	if rec := e.loginFrom(t, "192.0.2.9", "100000", wrongPassword, hdr); rec.Code != 401 {
		t.Fatalf("failure: %d", rec.Code)
	}
	if n := e.ipFailures(t, "198.51.100.1"); n != 1 || e.ipFailures(t, "192.0.2.9") != 0 || e.ipFailures(t, "10.9.9.9") != 0 {
		t.Fatalf("failure not recorded for the forwarded client only")
	}
	e.seed(t, "198.51.100.1", 29, 0)
	if rec := e.loginFrom(t, "192.0.2.9", "999999", wrongPassword, hdr); rec.Code != 429 {
		t.Fatalf("forwarded client not throttled: %d", rec.Code)
	}
	// Changing the spoofable prefix does not help; another proxy peer with another client is free.
	if rec := e.loginFrom(t, "192.0.2.9", "999999", wrongPassword,
		map[string]string{"X-Forwarded-For": "1.1.1.1, 198.51.100.1"}); rec.Code != 429 {
		t.Fatalf("spoofed prefix: %d", rec.Code)
	}
	if rec := e.loginFrom(t, "192.0.2.9", "999999", wrongPassword,
		map[string]string{"X-Forwarded-For": "198.51.100.2"}); rec.Code != 401 {
		t.Fatalf("other client: %d", rec.Code)
	}
}
