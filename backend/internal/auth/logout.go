package auth

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Logouts implements POST /auth/logout.
type Logouts struct {
	Sessions *Sessions
	Audit    Recorder
}

// Logout deletes the caller's session and writes AUTH_LOGOUT in the same
// transaction (fail-closed). Without a session there is nothing to end and no
// actor to record: it succeeds silently (PRD §6.5, idempotent 204). A session
// that vanished meanwhile (concurrent logout) is not an error either.
func (l *Logouts) Logout(ctx context.Context, sessionID string, p security.Principal, has bool, req Request) error {
	if !has {
		return nil
	}
	_, err := db.InTx(ctx, l.Sessions.DB, func(ctx context.Context, tx *sql.Tx) error {
		if err := l.Sessions.RevokeIn(ctx, tx, sessionID); err != nil {
			return err
		}
		return l.Audit.Record(ctx, tx, audit.Event{
			Action: audit.AuthLogout, Outcome: audit.Success,
			ActorRole: p.ActorRole(), ActorEmpNo: p.EmpNo,
			TargetType: audit.TargetUser, TargetID: p.UserID,
			Method: req.Method, Route: req.Route, Status: http.StatusNoContent,
			IP: req.IP, UserAgent: req.UserAgent,
		})
	})
	return err
}

// LogoutHandler is the HTTP adapter of Logouts.
type LogoutHandler struct {
	Logouts *Logouts
	Fail    func(w http.ResponseWriter, r *http.Request, err error)
}

// Logout serves POST /auth/logout: 204 and an expired session cookie, with or
// without a session. The CSRF check has already run in the middleware.
func (h LogoutHandler) Logout(w http.ResponseWriter, r *http.Request) {
	p, has := security.PrincipalFrom(r.Context())
	var sid string
	if c, err := r.Cookie(SessionCookie); err == nil {
		sid = c.Value
	}
	if err := h.Logouts.Logout(r.Context(), sid, p, has, Request{
		Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r),
	}); err != nil {
		h.Fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
