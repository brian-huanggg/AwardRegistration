package server

import (
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
)

// API composes the module handlers into api.ServerInterface. Operations whose
// module has not landed yet stay api.Unimplemented (501).
type API struct {
	api.Unimplemented
	Auth     auth.Handler
	Logins   auth.LoginHandler
	Logouts  auth.LogoutHandler
	Me       auth.MeHandler
	Account  account.Handler
	Deadline deadline.Handler
}

// GetDeadline serves GET /deadline.
func (a API) GetDeadline(w http.ResponseWriter, r *http.Request) { a.Deadline.GetDeadline(w, r) }

// Register serves POST /auth/register.
func (a API) Register(w http.ResponseWriter, r *http.Request, _ api.RegisterParams) {
	a.Account.Register(w, r)
}

// GetCsrfToken serves GET /auth/csrf.
func (a API) GetCsrfToken(w http.ResponseWriter, r *http.Request) { a.Auth.GetCsrfToken(w, r) }

// Login serves POST /auth/login.
func (a API) Login(w http.ResponseWriter, r *http.Request, _ api.LoginParams) { a.Logins.Login(w, r) }

// Logout serves POST /auth/logout.
func (a API) Logout(w http.ResponseWriter, r *http.Request, _ api.LogoutParams) {
	a.Logouts.Logout(w, r)
}

// GetMe serves GET /auth/me.
func (a API) GetMe(w http.ResponseWriter, r *http.Request) { a.Me.GetMe(w, r) }
