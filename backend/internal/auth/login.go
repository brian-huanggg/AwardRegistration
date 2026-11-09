package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

const (
	// bcryptCost is the only cost the table accepts (PRD §6.4, OI-13); the
	// dummy hash uses it too, so unknown and known accounts cost the same.
	bcryptCost = 10
	// bcryptMaxBytes is bcrypt's input limit. Longer input is cut for the dummy
	// comparison and never matches (ADR G3).
	bcryptMaxBytes = 72

	maxLoginBody = 16 << 10 // error-codes.md §2: 16 KiB → 413
)

var empNoFormat = regexp.MustCompile(`^[0-9]{6}$`)

// Throttle is the login throttling hook (PRD §6.5); DBThrottle implements it on
// login_lock. Errors fail the login closed (500): an unreachable counter must
// not disable the limit.
type Throttle interface {
	// Check runs before any password work. A positive retryAfter means the
	// attempt is throttled: it answers 429, runs no bcrypt and is not counted.
	// key is the submitted employee-ID string as typed; implementations strip
	// control characters and cut it to 32 characters.
	Check(ctx context.Context, key string, ip netip.Addr) (retryAfter time.Duration, err error)
	// Fail counts one failed attempt (also for unknown accounts).
	Fail(ctx context.Context, key string, ip netip.Addr) error
	// Succeed clears the key's counter.
	Succeed(ctx context.Context, key string) error
}

// NoThrottle never throttles.
type NoThrottle struct{}

// Check implements Throttle.
func (NoThrottle) Check(context.Context, string, netip.Addr) (time.Duration, error) { return 0, nil }

// Fail implements Throttle.
func (NoThrottle) Fail(context.Context, string, netip.Addr) error { return nil }

// Succeed implements Throttle.
func (NoThrottle) Succeed(context.Context, string) error { return nil }

// Recorder writes the success audit row in the login transaction.
// *audit.Service implements it.
type Recorder interface {
	Record(ctx context.Context, tx *sql.Tx, e audit.Event) error
}

// Logins implements POST /auth/login.
type Logins struct {
	Sessions *Sessions
	Audit    Recorder
	Throttle Throttle

	dummyHash []byte
}

// NewLogins builds the service and computes the dummy bcrypt hash once
// (ADR G3), so a request never pays for it and unknown accounts cost one
// cost-10 comparison like known ones.
func NewLogins(s *Sessions, rec Recorder, t Throttle) (*Logins, error) {
	if t == nil {
		t = NoThrottle{}
	}
	h, err := bcrypt.GenerateFromPassword([]byte("dummy password for timing"), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("dummy hash: %w", err)
	}
	return &Logins{Sessions: s, Audit: rec, Throttle: t, dummyHash: h}, nil
}

// LoginCommand is the raw input.
type LoginCommand struct{ EmpNo, Password string }

// Request is the transport facts the audit row needs.
type Request struct {
	Method, Route, UserAgent string
	IP                       netip.Addr
}

// LoggedIn is the result of a successful login.
type LoggedIn struct {
	SessionID string
	EmpNo     string
	Name      string
	Email     *string
	Role      security.Role
}

// Login checks the credentials and opens a session. Every refusal is a
// *web.Error carrying the AUTH_LOGIN audit fields (FAILED or THROTTLED), so the
// central adapter writes the track-B row; success writes its row in the same
// transaction as the session (fail-closed).
//
// Unknown, badly formatted and disabled accounts and wrong passwords answer
// the same 401 after the same one bcrypt comparison.
func (l *Logins) Login(ctx context.Context, cmd LoginCommand, req Request) (LoggedIn, error) {
	retry, err := l.Throttle.Check(ctx, cmd.EmpNo, req.IP)
	if err != nil {
		return LoggedIn{}, fmt.Errorf("login throttle check: %w", err)
	}
	if retry > 0 {
		e := web.NewError(web.CodeLoginThrottled, "too many login attempts")
		e.RetryAfter = retry
		return LoggedIn{}, loginRefused(e, audit.Throttled, "throttled", cmd.EmpNo)
	}

	var (
		acct   store.Account
		found  bool
		reason = "bad_format"
	)
	if empNoFormat.MatchString(cmd.EmpNo) {
		if acct, found, err = store.AccountByEmpNo(ctx, l.Sessions.DB, cmd.EmpNo); err != nil {
			return LoggedIn{}, err
		}
		reason = "unknown_account"
	}
	hash := l.dummyHash
	if found {
		hash, reason = []byte(acct.PasswordHash), ""
		if !acct.Enabled {
			hash, reason = l.dummyHash, "disabled"
		}
	}
	ok := comparePassword(hash, cmd.Password)
	switch {
	case reason != "":
	case len(cmd.Password) > bcryptMaxBytes:
		reason = "password_too_long"
	case !ok:
		reason = "wrong_password"
	}
	if reason != "" {
		if err := l.Throttle.Fail(ctx, cmd.EmpNo, req.IP); err != nil {
			return LoggedIn{}, fmt.Errorf("login throttle fail: %w", err)
		}
		return LoggedIn{}, loginRefused(web.NewError(web.CodeInvalidCredentials, "invalid employee ID or password"),
			audit.Failed, reason, cmd.EmpNo)
	}

	if err := l.Throttle.Succeed(ctx, cmd.EmpNo); err != nil {
		return LoggedIn{}, fmt.Errorf("login throttle clear: %w", err)
	}
	var sid string
	_, err = db.InTx(ctx, l.Sessions.DB, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		if sid, err = l.Sessions.CreateIn(ctx, tx, acct.ID); err != nil {
			return err
		}
		return l.Audit.Record(ctx, tx, audit.Event{
			Action: audit.AuthLogin, Outcome: audit.Success,
			ActorRole: security.Principal{Role: acct.Role}.ActorRole(), ActorEmpNo: acct.EmpNo,
			TargetType: audit.TargetUser, TargetID: acct.ID,
			Method: req.Method, Route: req.Route, Status: http.StatusOK,
			IP: req.IP, UserAgent: req.UserAgent,
		})
	})
	if err != nil {
		return LoggedIn{}, err
	}
	return LoggedIn{SessionID: sid, EmpNo: acct.EmpNo, Name: acct.Name, Email: acct.Email, Role: acct.Role}, nil
}

// comparePassword runs exactly one cost-10 comparison. Input beyond bcrypt's
// 72 bytes is cut for the comparison against the dummy hash and never
// succeeds: newer x/crypto refuses such input before doing any work, which
// would make it fast and so distinguishable (ADR G3).
func comparePassword(hash []byte, password string) bool {
	pw := []byte(password)
	if len(pw) > bcryptMaxBytes {
		_ = bcrypt.CompareHashAndPassword(hash, pw[:bcryptMaxBytes])
		return false
	}
	return bcrypt.CompareHashAndPassword(hash, pw) == nil
}

func loginRefused(e *web.Error, outcome audit.Outcome, reason, submitted string) *web.Error {
	e.AuditAction = string(audit.AuthLogin)
	e.AuditOutcome = string(outcome)
	e.AuditReason = reason
	e.AuditSubmittedEmpNo = submitted
	return e
}

// loginBody holds pointers so DecodeJSON can tell absent/null (400) from empty.
type loginBody struct {
	EmpNo    *string `json:"empNo"`
	Password *string `json:"password"`
}

// LoginHandler is the HTTP adapter of Logins. Fail is the central error
// adapter (server.ErrorAdapter.Write); the module cannot import server.
type LoginHandler struct {
	Logins *Logins
	Fail   func(w http.ResponseWriter, r *http.Request, err error)
}

// Login serves POST /auth/login: 200 {empNo, name, email, role} and the
// session cookie.
func (h LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	var b loginBody
	if err := web.DecodeJSON(r, &b); err != nil {
		h.Fail(w, r, auditMalformed(err))
		return
	}
	res, err := h.Logins.Login(r.Context(), LoginCommand{EmpNo: *b.EmpNo, Password: *b.Password}, Request{
		Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r),
	})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: res.SessionID, Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	web.WriteJSON(w, http.StatusOK, api.LoginResponse{
		EmpNo: res.EmpNo, Name: res.Name, Email: res.Email, Role: api.LoginResponseRole(res.Role),
	})
}

// auditMalformed marks a 400 as a FAILED login. Other transport errors (413,
// 415) are not audited.
func auditMalformed(err error) error {
	var e *web.Error
	if errors.As(err, &e) && e.Code == web.CodeMalformedRequest {
		return loginRefused(e, audit.Failed, "malformed_request", "")
	}
	return err
}
