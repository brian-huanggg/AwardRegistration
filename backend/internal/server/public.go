package server

import (
	"net/http"
	"strings"
)

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
	http.MethodPut + " /api/v1/admin/deadline": "DEADLINE_UPDATE",
}

// ideaPrefix is the path of an Idea resource without its ID.
const ideaPrefix = "/api/v1/ideas/"

// ActionFor returns the audit action of r, or "" when unknown. PUT on one Idea
// (/api/v1/ideas/{id}, a single segment) is IDEA_UPDATE and PUT on its slides
// (/api/v1/ideas/{id}/slides) is SLIDES_REPLACE, whatever the ID says:
// a CSRF denial happens before the ID is parsed.
func ActionFor(r *http.Request) string {
	if a, ok := WriteActions[r.Method+" "+r.URL.Path]; ok {
		return a
	}
	if id, ok := strings.CutPrefix(r.URL.Path, ideaPrefix); ok && id != "" && r.Method == http.MethodPut {
		if !strings.Contains(id, "/") {
			return "IDEA_UPDATE"
		}
		if i, ok := strings.CutSuffix(id, "/slides"); ok && i != "" && !strings.Contains(i, "/") {
			return "SLIDES_REPLACE"
		}
	}
	return ""
}
