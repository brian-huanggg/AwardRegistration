package server

import "net/http"

// PublicRoutes is the explicit list of operations that need no session
// (Backend.md §3). It must equal the operations marked `security: []` in
// api/openapi.yaml; a test locks that. B1-10 adds POST /auth/login to both. Paths include the /api/v1 prefix.
var PublicRoutes = []struct{ Method, Path string }{
	{http.MethodGet, "/api/v1/auth/csrf"},
	{http.MethodPost, "/api/v1/auth/register"},
	{http.MethodPost, "/api/v1/auth/logout"},
	{http.MethodGet, "/api/v1/deadline"},
}

// IsPublic reports whether r is exactly one of PublicRoutes. A public path with
// another method is not public: without a session it answers 401, with one 405.
func IsPublic(r *http.Request) bool {
	for _, p := range PublicRoutes {
		if r.Method == p.Method && r.URL.Path == p.Path {
			return true
		}
	}
	return false
}
