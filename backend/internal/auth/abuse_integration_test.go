//go:build integration

package auth_test

// Abuse-case tests: attacks from OWASP WSTG (WSTG-ATHN-03 lockout,
// WSTG-ATHN-04 bypass, WSTG-SESS-05 CSRF, WSTG-SESS-01 session tokens,
// WSTG-IDNT-04 account enumeration) encoded as repeatable tests. Each test
// names the attack it simulates; a failure is a security finding, not flake.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

// burst fires n concurrent login requests and returns how many got each status.
// It reuses one anonymous CSRF token (anonymous tokens are not single-use), so
// the goroutines never call t.Fatal.
func (e loginEnv) burst(t *testing.T, n int, body func(i int) string) map[int]int {
	t.Helper()
	tok, ck := e.csrf(t, nil)
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		counts = map[int]int{}
		start  = make(chan struct{})
	)
	for i := range n {
		wg.Go(func() {
			<-start // release together: maximise the Check→Fail overlap
			rec := e.loginWith(body(i), tok, ck)
			mu.Lock()
			counts[rec.Code]++
			mu.Unlock()
		})
	}
	close(start)
	wg.Wait()
	return counts
}

// Attack: parallel password guessing against one account (single-packet /
// Turbo Intruder style) to get more than 5 guesses per lock window.
// Every 401 is one password actually checked.
func TestAbuseConcurrentBurstCannotExceedKeyLimit(t *testing.T) {
	e := newLockEnv(t)
	e.addUser(t, "123456", "USER", true)

	got := e.burst(t, 50, func(int) string { return creds("123456", wrongPassword) })
	if got[401] > 5 {
		t.Errorf("50 concurrent guesses: %d passwords checked, PRD §6.5 allows 5 per window (statuses %v)", got[401], got)
	}
	if got[401]+got[429] != 50 {
		t.Errorf("unexpected statuses: %v", got)
	}
}

// Attack: parallel credential stuffing from one address across many accounts
// to get past the 30-per-15-minutes IP limit.
func TestAbuseConcurrentBurstCannotExceedIPLimit(t *testing.T) {
	e := newLockEnv(t)

	got := e.burst(t, 90, func(i int) string { return creds(fmt.Sprintf("9%05d", i), wrongPassword) })
	if got[401] > 30 {
		t.Errorf("90 concurrent attempts from one IP: %d passwords checked, PRD §6.5 allows 30 per window (statuses %v)", got[401], got)
	}
}

// Attack: an attacker with their own session obtains a valid CSRF token and
// plants it (header + cookie) in a request that carries the victim's session.
func TestAbuseCSRFTokenFromAnotherSessionIsRejected(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "111111", "USER", true)
	e.addUser(t, "222222", "USER", true)
	attacker := findCookie(e.login(t, creds("111111", password)), auth.SessionCookie)
	victim := findCookie(e.login(t, creds("222222", password)), auth.SessionCookie)

	atkTok, atkCk := e.csrf(t, attacker)
	vicTok, vicCk := e.csrf(t, victim)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"attacker token+cookie":          e.logout(atkTok, atkCk, victim),
		"attacker header, own cookie":    e.logout(atkTok, vicCk, victim),
		"victim header, attacker cookie": e.logout(vicTok, atkCk, victim),
	} {
		if rec.Code != 403 || code(t, rec) != "CSRF_INVALID" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if e.sessions(t) != 2 {
		t.Error("a rejected cross-session write ended a session")
	}
}

// Attack: forge a CSRF token by editing a valid one (nonce or signature).
func TestAbuseTamperedCSRFTokenIsRejected(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	sess := findCookie(e.login(t, creds("123456", password)), auth.SessionCookie)
	tok, _ := e.csrf(t, sess)
	nonce, sig, _ := strings.Cut(tok, ".")

	flip := func(s string, i int) string {
		b := []byte(s)
		if b[i] == 'A' {
			b[i] = 'B'
		} else {
			b[i] = 'A'
		}
		return string(b)
	}
	for name, forged := range map[string]string{
		"nonce edited":     flip(nonce, 0) + "." + sig,
		"signature edited": nonce + "." + flip(sig, 0),
		"signature empty":  nonce + ".",
		"no separator":     nonce + sig,
		"extra segment":    tok + ".x",
	} {
		ck := &http.Cookie{Name: auth.CSRFCookie, Value: forged}
		if rec := e.logout(forged, ck, sess); rec.Code != 403 {
			t.Errorf("%s: %d, want 403", name, rec.Code)
		}
	}
	if e.sessions(t) != 1 {
		t.Error("a forged token ended the session")
	}
}

// Attack: guess or edit a session cookie (WSTG-SESS-01). Only the exact value
// issued at login may authenticate.
func TestAbuseTamperedSessionCookieIs401(t *testing.T) {
	e := newLoginEnv(t)
	e.addUser(t, "123456", "USER", true)
	sid := findCookie(e.login(t, creds("123456", password)), auth.SessionCookie).Value
	if rec := e.me(t, true, sid); rec.Code != 200 {
		t.Fatalf("baseline: %d", rec.Code)
	}

	first := "A"
	if sid[0] == 'A' {
		first = "B"
	}
	for name, v := range map[string]string{
		"first char edited": first + sid[1:],
		"truncated":         sid[:len(sid)-1],
		"extended":          sid + "A",
		"padded":            sid + "=",
		"std alphabet":      strings.NewReplacer("-", "+", "_", "/").Replace(sid) + "+",
		"all zero":          strings.Repeat("A", len(sid)),
		"sql meta":          "' OR '1'='1",
		"oversized":         strings.Repeat("A", 4096),
	} {
		if rec := e.me(t, true, v); rec.Code != 401 {
			t.Errorf("%s: %d, want 401", name, rec.Code)
		}
	}
}

// Attack: account enumeration by response time (WSTG-IDNT-04). Unknown,
// badly formatted, disabled and known accounts must all cost one bcrypt
// comparison, so their median latencies stay close.
func TestAbuseLoginTimingDoesNotRevealAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	e := newLoginEnv(t) // fake throttle: no lock interferes with sampling
	e.addUser(t, "123456", "USER", true)
	e.addUser(t, "654321", "USER", false)
	tok, ck := e.csrf(t, nil)

	const samples = 15
	median := func(body string) time.Duration {
		d := make([]time.Duration, samples)
		for i := range d {
			start := time.Now()
			if rec := e.loginWith(body, tok, ck); rec.Code != 401 {
				t.Fatalf("%s: %d", body, rec.Code)
			}
			d[i] = time.Since(start)
		}
		slices.Sort(d)
		return d[samples/2]
	}
	base := median(creds("123456", wrongPassword))
	for name, body := range map[string]string{
		"unknown account":  creds("999999", wrongPassword),
		"bad format":       creds("abc", wrongPassword),
		"disabled account": creds("654321", password),
		"long password":    creds("123456", strings.Repeat("x", 200)),
	} {
		m := median(body)
		diff := max(m-base, base-m)
		if limit := max(20*time.Millisecond, base/4); diff > limit {
			t.Errorf("%s: median %v vs known account %v (diff %v > %v): timing reveals the account state", name, m, base, diff, limit)
		}
	}
}
