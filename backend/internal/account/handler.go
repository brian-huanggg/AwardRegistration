package account

import (
	"errors"
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// maxJSONBody is the JSON body limit (error-codes.md §2: 16 KiB → 413).
const maxJSONBody = 16 << 10

// Handler is the HTTP adapter of the account module. Fail is the central error
// adapter (server.ErrorAdapter.Write); the module cannot import server.
type Handler struct {
	Service *Service
	Fail    func(w http.ResponseWriter, r *http.Request, err error)
}

// registerBody holds pointers so DecodeJSON can tell absent/null (400) from
// empty (422). Unknown keys are ignored.
type registerBody struct {
	EmpNo    *string `json:"empNo"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
}

// Register serves POST /auth/register: 201 {empNo, name, email}, no cookie, no Location.
func (h Handler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var b registerBody
	if err := web.DecodeJSON(r, &b); err != nil {
		h.Fail(w, r, auditMalformed(err))
		return
	}
	u, err := h.Service.Register(r.Context(), RegisterCommand{
		EmpNo: *b.EmpNo, Name: *b.Name, Email: *b.Email, Password: *b.Password,
	}, Request{
		Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r),
	})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusCreated, api.RegisterResponse{EmpNo: u.EmpNo, Name: u.Name, Email: u.Email})
}

// auditMalformed marks a 400 as a FAILED registration (PRD §6.11 audits 400,
// 409 and 422 here). Other transport errors (413, 415) are not audited.
func auditMalformed(err error) error {
	var e *web.Error
	if errors.As(err, &e) && e.Code == web.CodeMalformedRequest {
		return failed(e, "malformed_request", "", false)
	}
	return err
}
