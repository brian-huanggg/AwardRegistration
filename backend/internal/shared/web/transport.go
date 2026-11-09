package web

import (
	"errors"
	"mime"
	"net/http"
	"strings"
)

// NotFound answers the unknown-path 404 as problem+json. (Without a session the
// session middleware answers 401 first; it is added in a later PR.)
func NotFound(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, NewError(CodeNotFound, "Resource not found."))
}

// MethodNotAllowed answers 405 as problem+json (also for HEAD on GET routes).
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, NewError(CodeMethodNotAllowed, "Method not allowed."))
}

// AcceptCheck answers 406 when the Accept header cannot be satisfied by JSON.
// A missing header, */*, application/* and application/json (also +problem)
// are accepted. Requests for which exempt reports true (routes that serve a
// file, not JSON) skip the check; exempt may be nil.
func AcceptCheck(next http.Handler, exempt func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (exempt == nil || !exempt(r)) && !acceptsJSON(r.Header.Values("Accept")) {
			WriteProblem(w, r, NewError(CodeNotAcceptable, "Only application/json is supported."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func acceptsJSON(values []string) bool {
	if len(values) == 0 {
		return true
	}
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			mt, params, err := mime.ParseMediaType(strings.TrimSpace(part))
			if err != nil {
				continue
			}
			if q := params["q"]; q == "0" || q == "0.0" {
				continue
			}
			switch mt {
			case "*/*", "application/*", "application/json", "application/problem+json":
				return true
			}
		}
	}
	return false
}

// RequireJSON answers 415 UNSUPPORTED_MEDIA_TYPE unless the request has a body
// with Content-Type application/json. Used by JSON routes only.
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength != 0 || r.Body != http.NoBody {
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				WriteProblem(w, r, NewError(CodeUnsupportedMedia, "Content-Type must be application/json."))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// LimitBody caps the request body. When the cap is exceeded the reader fails;
// handlers map that with IsBodyTooLarge. Over-limit declared lengths are
// answered immediately, before any other work and without reading the body.
func LimitBody(max int64, code ErrorCode) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > max {
				WriteProblem(w, r, NewError(code, "Request body too large."))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}

// IsBodyTooLarge reports whether err comes from http.MaxBytesReader.
func IsBodyTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}
