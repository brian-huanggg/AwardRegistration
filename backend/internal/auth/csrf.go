package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// CSRF names and sizes (PRD §6.5, Backend.md §6, D40).
const (
	// CSRFCookie is the double-submit cookie. The __Host- prefix forces Secure,
	// Path=/ and no Domain; HttpOnly keeps it away from script, which reads the
	// token from the GET /auth/csrf body instead.
	CSRFCookie = "__Host-csrf"
	// CSRFHeader carries the token on every write.
	CSRFHeader = "X-CSRF-Token"

	// MinCSRFKeyBytes is the shortest accepted HMAC key (the SHA-256 output size).
	MinCSRFKeyBytes = 32

	nonceBytes = 16
)

var anonymousBinding = []byte{0x00, 'a', 'n', 'o', 'n'}

// CSRF signs and verifies session-bound double-submit tokens:
//
//	base64url(nonce) "." base64url(HMAC-SHA256(key, binding ‖ nonce))
//
// The binding is 0x01 ‖ SHA-256(session ID) with a session, a fixed anonymous
// value without one. A token minted anonymously, or for another session,
// therefore fails against a victim's session (D40).
//
// The first key signs; every key verifies. Rotation: make the new key current
// and the old one previous, and drop the previous key after 8 h (Backend.md §6).
// Tokens carry no expiry, so the operator's removal of the old key is the bound.
type CSRF struct {
	keys [][]byte
}

// NewCSRF returns a signer. current signs; previous (optional) only verifies.
func NewCSRF(current string, previous ...string) (*CSRF, error) {
	c := &CSRF{}
	for i, k := range append([]string{current}, previous...) {
		if i > 0 && k == "" {
			continue // an unset previous key means "no rotation in progress"
		}
		if len(k) < MinCSRFKeyBytes {
			return nil, fmt.Errorf("csrf key %d: need at least %d bytes", i, MinCSRFKeyBytes)
		}
		c.keys = append(c.keys, []byte(k))
	}
	return c, nil
}

// bindingFor returns the binding for a request: the session's when the session
// middleware resolved one, else the anonymous value. ok is false only when a
// validated session carries an ID that cannot be bound (cannot happen unless
// the validator and Binding disagree).
func bindingFor(r *http.Request) (b []byte, ok bool) {
	if _, has := security.PrincipalFrom(r.Context()); !has {
		return anonymousBinding, true
	}
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return nil, false
	}
	h, ok := Binding(c.Value)
	if !ok {
		return nil, false
	}
	return append([]byte{0x01}, h...), true
}

func mac(key, binding, nonce []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(binding)
	m.Write(nonce)
	return m.Sum(nil)
}

// Issue returns a fresh token for binding.
func (c *CSRF) Issue(binding []byte) (string, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return b64(nonce) + "." + b64(mac(c.keys[0], binding, nonce)), nil
}

// Verify reports whether token was issued by one of the keys for binding.
func (c *CSRF) Verify(token string, binding []byte) bool {
	n, s, found := strings.Cut(token, ".")
	if !found {
		return false
	}
	nonce, err1 := base64.RawURLEncoding.DecodeString(n)
	sig, err2 := base64.RawURLEncoding.DecodeString(s)
	if err1 != nil || err2 != nil || len(nonce) != nonceBytes {
		return false
	}
	ok := false
	for _, k := range c.keys { // no early exit: time does not tell which key matched
		if hmac.Equal(sig, mac(k, binding, nonce)) {
			ok = true
		}
	}
	return ok
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// safeMethod is the set that is never CSRF-checked (RFC 9110 §9.2.1).
func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// ActionFor names the audit action of the write r attempts, or "" when it is
// unknown (unrouted path). A rejected attempt is audited only with an action.
type ActionFor func(r *http.Request) string

// Middleware enforces the CSRF check on every non-safe method. It runs after
// the session middleware (so 401 wins) and before routing (so 403 beats 404),
// reads only the header and the cookie and never the body. Accepted only if
// cookie and header are both present and equal, the signature verifies, and
// the binding matches the request's session (or anonymous without one).
//
// fail is the central error adapter; it audits the denial when the *web.Error
// carries an AuditReason and an action.
func (c *CSRF) Middleware(fail func(http.ResponseWriter, *http.Request, error), action ActionFor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if safeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if reason := c.check(r); reason != "" {
				e := web.NewError(web.CodeCSRFInvalid, "CSRF token missing or invalid.")
				if action != nil {
					if a := action(r); a != "" {
						e.AuditAction, e.AuditReason = a, reason
					}
				}
				fail(w, r, e)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// check returns "" when r passes, else the audit reason (never sent to the client).
func (c *CSRF) check(r *http.Request) string {
	hdr := r.Header.Get(CSRFHeader)
	ck, err := r.Cookie(CSRFCookie)
	switch {
	case hdr == "" || err != nil || ck.Value == "":
		return "csrf_missing"
	case subtle.ConstantTimeCompare([]byte(hdr), []byte(ck.Value)) != 1:
		return "csrf_mismatch"
	}
	b, ok := bindingFor(r)
	if !ok || !c.Verify(hdr, b) {
		return "csrf_invalid_binding"
	}
	return ""
}

// Handler serves GET /auth/csrf: a token for the caller's current binding in
// the body and in the cookie. It is public; with a session the token is bound
// to it. Login and logout change the binding, so the client refetches then.
type Handler struct {
	CSRF *CSRF
	Log  *slog.Logger
}

// GetCsrfToken serves GET /auth/csrf.
func (h Handler) GetCsrfToken(w http.ResponseWriter, r *http.Request) {
	b, ok := bindingFor(r)
	if !ok {
		h.fail(w, r, errors.New("session without bindable ID"))
		return
	}
	tok, err := h.CSRF.Issue(b)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: CSRFCookie, Value: tok, Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	web.WriteJSON(w, http.StatusOK, api.CsrfTokenResponse{Token: tok})
}

func (h Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.Log.ErrorContext(r.Context(), "csrf token issue failed",
		"requestId", web.RequestIDFrom(r.Context()), "error", err)
	web.WriteProblem(w, r, web.NewError(web.CodeInternalError, ""))
}
