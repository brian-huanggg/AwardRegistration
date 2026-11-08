// Package apispec validates HTTP responses against api/openapi.yaml
// (Backend.md §11). Integration tests wrap their handler with Validate so
// that every response, success or problem+json, is checked against the
// contract; a wrong or missing field fails the test.
package apispec

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// Spec is a loaded OpenAPI document with its router.
type Spec struct {
	router routers.Router
}

// Load reads api/openapi.yaml of this module.
func Load(tb testing.TB) *Spec {
	tb.Helper()
	_, file, _, _ := runtime.Caller(0)
	return LoadFile(tb, filepath.Join(filepath.Dir(file), "..", "..", "api", "openapi.yaml"))
}

// LoadFile reads and validates the document at path.
func LoadFile(tb testing.TB, path string) *Spec {
	tb.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		tb.Fatalf("apispec: load %s: %v", path, err)
	}
	if err := doc.Validate(openapi3.NewLoader().Context); err != nil {
		tb.Fatalf("apispec: %s is not a valid OpenAPI document: %v", path, err)
	}
	r, err := legacy.NewRouter(doc)
	if err != nil {
		tb.Fatalf("apispec: router: %v", err)
	}
	return &Spec{router: r}
}

// Check validates one recorded response of req. It returns an error when the
// operation is not in the spec, or the status, headers or body do not match.
func (s *Spec) Check(req *http.Request, res *http.Response, body []byte) error {
	route, pathParams, err := s.router.FindRoute(req)
	if err != nil {
		return err
	}
	return openapi3filter.ValidateResponse(req.Context(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req, PathParams: pathParams, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status:  res.StatusCode,
		Options: &openapi3filter.Options{IncludeResponseStatus: true}, // undeclared status = failure
		Header:  res.Header,
		Body:    nopCloser{bytes.NewReader(body)},
	})
}

// Validate wraps h: every response it writes is checked against the spec and
// a mismatch fails the test (Errorf, so the response is still delivered).
func (s *Spec) Validate(tb testing.TB, h http.Handler) http.Handler {
	tb.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		res := rec.Result()
		body := rec.Body.Bytes()
		if err := s.Check(req, res, body); err != nil {
			tb.Errorf("%s %s: response %d does not match api/openapi.yaml: %v\nbody: %s",
				req.Method, req.URL.Path, res.StatusCode, err, body)
		}
		for k, v := range res.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(res.StatusCode)
		_, _ = w.Write(body)
	})
}

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }
