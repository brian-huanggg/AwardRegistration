package web

import (
	"log/slog"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// NewLogger returns a JSON slog logger using ECS field names.
func NewLogger(w interface{ Write([]byte) (int, error) }, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "@timestamp"
			case slog.LevelKey:
				a.Key = "log.level"
			case slog.MessageKey:
				a.Key = "message"
			}
			return a
		},
	}))
}

// AccessLog writes one ECS-style line per request. Query strings, headers and
// bodies are never logged (they may carry tokens). Must sit inside RequestID.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			log.InfoContext(r.Context(), "http request",
				"requestId", RequestIDFrom(r.Context()),
				"http.request.method", r.Method,
				"url.path", r.URL.Path,
				"http.response.status_code", rec.status,
				"http.response.body.bytes", rec.bytes,
				"event.duration", time.Since(start).Nanoseconds())
		})
	}
}
