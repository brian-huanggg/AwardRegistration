package server

import (
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
)

// API composes the module handlers into api.ServerInterface. Operations whose
// module has not landed yet stay api.Unimplemented (501).
type API struct {
	api.Unimplemented
	Account  account.Handler
	Deadline deadline.Handler
}

// GetDeadline serves GET /deadline.
func (a API) GetDeadline(w http.ResponseWriter, r *http.Request) { a.Deadline.GetDeadline(w, r) }

// Register serves POST /auth/register.
func (a API) Register(w http.ResponseWriter, r *http.Request, _ api.RegisterParams) {
	a.Account.Register(w, r)
}
