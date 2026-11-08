package web

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover turns a panic into a 500 INTERNAL_ERROR problem. The panic value and
// stack go to the log only, never to the client.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler { //nolint:errorlint // sentinel panic value, not an error chain
					panic(rec)
				}
				log.ErrorContext(r.Context(), "panic recovered",
					"requestId", RequestIDFrom(r.Context()),
					"panic", rec, "stack", string(debug.Stack()))
				WriteProblem(w, r, NewError(CodeInternalError, ""))
			}()
			next.ServeHTTP(w, r)
		})
	}
}
