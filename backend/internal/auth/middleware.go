package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// SessionCookie is the session cookie name (openapi sessionCookie). The
// __Host- prefix forces Secure, Path=/ and no Domain.
const SessionCookie = "__Host-session"

// Validator resolves a cookie value to its caller. *Sessions implements it.
type Validator interface {
	Validate(ctx context.Context, id string) (p security.Principal, ok bool, err error)
}

// Public reports whether a request may be served without a session. It is
// decided on method and path before routing, so unknown paths are not public.
type Public func(r *http.Request) bool

// Middleware is the session middleware (Backend.md §3, §6). It must wrap the
// router, not sit inside it, so that an unknown path without a session answers
// 401 and only a caller with a session learns it is a 404 (PRD §6.1).
//
//   - Public request: a valid session is still resolved (the principal is then
//     in the context for the audit actor and the session-bound CSRF token); a
//     missing or invalid one is simply anonymous.
//   - Any other request: no valid session → 401 UNAUTHENTICATED. With a valid
//     session the principal is stored in the context and routing continues, so
//     404 and 405 come from the router.
//
// It never reads the body. A database failure answers 500, never a guess.
func Middleware(v Validator, public Public, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isPublic := public != nil && public(r)

			var (
				p  security.Principal
				ok bool
			)
			if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
				var verr error
				if p, ok, verr = v.Validate(r.Context(), c.Value); verr != nil {
					log.ErrorContext(r.Context(), "session validation failed",
						"requestId", web.RequestIDFrom(r.Context()), "error", verr)
					web.WriteProblem(w, r, web.NewError(web.CodeInternalError, ""))
					return
				}
			}

			switch {
			case ok:
				r = r.WithContext(security.WithPrincipal(r.Context(), p))
			case !isPublic:
				web.WriteProblem(w, r, web.NewError(web.CodeUnauthenticated, "Authentication required."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
