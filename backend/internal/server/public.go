package server

import "net/http"

// PublicRoutes is the explicit list of operations that need no session
// (Backend.md §3). It must equal the operations marked `security: []` in
// api/openapi.yaml; a test locks that. Paths include the /api/v1 prefix.
var PublicRoutes = []struct{ Method, Path string }{
	{http.MethodGet, "/api/v1/auth/csrf"},
	{http.MethodPost, "/api/v1/auth/login"},
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

// WriteActions maps the writes that exist today to their audit action, so a
// CSRF denial is recorded (PRD §6.11). Later PRs add their routes here, with
// the same exact-path rule; a write on an unknown path is not audited.
var WriteActions = map[string]string{
	http.MethodPost + " /api/v1/auth/login":    "AUTH_LOGIN",
	http.MethodPost + " /api/v1/auth/register": "AUTH_REGISTER",
	http.MethodPost + " /api/v1/auth/logout":   "AUTH_LOGOUT",
	http.MethodPost + " /api/v1/ideas":         "IDEA_CREATE",
}

// ActionFor returns the audit action of r, or "" when unknown.
func ActionFor(r *http.Request) string { return WriteActions[r.Method+" "+r.URL.Path] }
