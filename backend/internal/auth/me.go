package auth

import (
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// MeHandler serves GET /auth/me. The session middleware has already answered
// 401 without a valid session and touched the session (idle refresh, OI-11),
// so this is also the idle-timeout "continue" call (OI-22).
type MeHandler struct {
	Sessions *Sessions
	Fail     func(w http.ResponseWriter, r *http.Request, err error)
}

// GetMe answers the caller from the database (name and email are not in the
// principal). An account that vanished since the middleware ran answers 401.
func (h MeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	a, found, err := store.AccountByID(r.Context(), h.Sessions.DB, p.UserID)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	if !found || !a.Enabled {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	web.WriteJSON(w, http.StatusOK, api.LoginResponse{
		EmpNo: a.EmpNo, Name: a.Name, Email: a.Email, Role: api.LoginResponseRole(a.Role),
	})
}
