package team

import (
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// maxJSONBody is the JSON body limit (error-codes.md §2: 16 KiB → 413).
const maxJSONBody = 16 << 10

// Handler serves the team endpoints.
type Handler struct {
	Service *Service
	Fail    func(w http.ResponseWriter, r *http.Request, err error)
}

// joinBody holds pointers so DecodeJSON can tell absent/null (400) from empty
// (422). Unknown keys are ignored.
type joinBody struct {
	LeaderEmpNo *string `json:"leaderEmpNo"`
	IdeaName    *string `json:"ideaName"`
}

type person struct {
	EmpNo string `json:"empNo"`
	Name  string `json:"name"`
}

// joinRequest is JoinRequest (docs/API/response-bodies.md).
type joinRequest struct {
	IdeaID      string `json:"ideaId"`
	IdeaName    string `json:"ideaName"`
	Category    string `json:"category"`
	Org         string `json:"org"`
	Direction   string `json:"direction"`
	Leader      person `json:"leader"`
	RequestedAt string `json:"requestedAt"`
}

func toBody(j JoinRequest) joinRequest {
	return joinRequest{
		IdeaID: j.IdeaID.String(), IdeaName: j.IdeaName, Category: j.Category, Org: j.Org, Direction: j.Direction,
		Leader:      person{EmpNo: j.LeaderEmpNo, Name: j.LeaderName},
		RequestedAt: web.FormatTime(j.RequestedAt),
	}
}

// ListJoinRequests serves GET /join-requests: {items: JoinRequest[]}.
func (h Handler) ListJoinRequests(w http.ResponseWriter, r *http.Request) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	got, err := h.Service.ListMine(r.Context(), p)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	items := make([]joinRequest, 0, len(got))
	for _, j := range got {
		items = append(items, toBody(j))
	}
	web.WriteJSON(w, http.StatusOK, struct {
		Items []joinRequest `json:"items"`
	}{items})
}

// CreateJoinRequest serves POST /join-requests: 201 + JoinRequest. Role and
// state run before the body is read (Backend.md §3).
func (h Handler) CreateJoinRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	if err := h.Service.AuthorizeApply(r.Context(), p); err != nil {
		h.Fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var b joinBody
	if err := web.DecodeJSON(r, &b); err != nil {
		h.Fail(w, r, err)
		return
	}
	got, err := h.Service.Apply(r.Context(), p, *b.LeaderEmpNo, *b.IdeaName,
		Request{Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r)})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusCreated, toBody(got))
}
