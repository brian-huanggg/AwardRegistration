package server

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/team"
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
	Ideas    idea.Handler
	Team     team.Handler
}

// GetDeadline serves GET /deadline.
func (a API) GetDeadline(w http.ResponseWriter, r *http.Request) { a.Deadline.GetDeadline(w, r) }

// PutDeadline serves PUT /admin/deadline.
func (a API) PutDeadline(w http.ResponseWriter, r *http.Request, _ api.PutDeadlineParams) {
	a.Deadline.PutDeadline(w, r)
}

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

// GetIdea serves GET /ideas/{id}.
func (a API) GetIdea(w http.ResponseWriter, r *http.Request, id api.IdeaId) {
	a.Ideas.GetIdea(w, r, id)
}

// CreateIdea serves POST /ideas.
func (a API) CreateIdea(w http.ResponseWriter, r *http.Request, _ api.CreateIdeaParams) {
	a.Ideas.CreateIdea(w, r)
}

// ListIdeas serves GET /ideas.
func (a API) ListIdeas(w http.ResponseWriter, r *http.Request, params api.ListIdeasParams) {
	a.Ideas.ListIdeas(w, r, params)
}

// DownloadSlides serves GET /ideas/{id}/slides.
func (a API) DownloadSlides(w http.ResponseWriter, r *http.Request, id api.IdeaId) {
	a.Ideas.DownloadSlides(w, r, id)
}

// UpdateIdea serves PUT /ideas/{id}.
func (a API) UpdateIdea(w http.ResponseWriter, r *http.Request, id api.IdeaId, _ api.UpdateIdeaParams) {
	a.Ideas.UpdateIdea(w, r, uuid.UUID(id))
}

// ReplaceSlides serves PUT /ideas/{id}/slides.
func (a API) ReplaceSlides(w http.ResponseWriter, r *http.Request, id api.IdeaId, _ api.ReplaceSlidesParams) {
	a.Ideas.ReplaceSlides(w, r, uuid.UUID(id))
}

// DeleteIdea serves DELETE /ideas/{id}.
func (a API) DeleteIdea(w http.ResponseWriter, r *http.Request, id api.IdeaId, _ api.DeleteIdeaParams) {
	a.Ideas.DeleteIdea(w, r, uuid.UUID(id))
}

// CreateJoinRequest serves POST /join-requests.
func (a API) CreateJoinRequest(w http.ResponseWriter, r *http.Request, _ api.CreateJoinRequestParams) {
	a.Team.CreateJoinRequest(w, r)
}
