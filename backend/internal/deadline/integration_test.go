//go:build integration

package deadline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

// TestGetDeadlineIntegration runs the real handler against MariaDB with the
// real V001 migration and validates each response against api/openapi.yaml.
func TestGetDeadlineIntegration(t *testing.T) {
	ctx := context.Background()
	db := dbtest.New(t)

	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := doc.Paths.Find("/deadline")
	route := &routers.Route{Spec: doc, Path: "/deadline", PathItem: path, Method: "GET", Operation: path.Get}

	h := server.NewPublic(slog.New(slog.DiscardHandler),
		server.Mount(server.API{Deadline: deadline.Handler{Store: deadline.SQLStore{DB: db}}}))

	call := func(t *testing.T) (b struct {
		Deadline   *string
		State      string
		ServerTime string
	}) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/deadline", http.NoBody)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		err := openapi3filter.ValidateResponse(ctx, &openapi3filter.ResponseValidationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{
				Request: req, Route: route,
				Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
			},
			Status: rec.Code, Header: rec.Header(), Body: io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
		})
		if err != nil {
			t.Fatalf("response does not match the spec: %v", err)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		if _, err := time.Parse(time.RFC3339Nano, b.ServerTime); err != nil || len(b.ServerTime) != len("2006-01-02T15:04:05.000Z") {
			t.Fatalf("serverTime %q is not millisecond UTC", b.ServerTime)
		}
		return b
	}

	t.Run("fresh database is NOT_OPEN", func(t *testing.T) {
		if b := call(t); b.Deadline != nil || b.State != "NOT_OPEN" {
			t.Fatalf("%+v", b)
		}
	})
	for _, tc := range []struct{ name, sql, state string }{
		{"future deadline is OPEN", "NOW() + INTERVAL 1 HOUR", "OPEN"},
		{"past deadline is CLOSED", "NOW() - INTERVAL 1 HOUR", "CLOSED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.ExecContext(ctx, "UPDATE contest_setting SET deadline = "+tc.sql+" WHERE id = 1"); err != nil {
				t.Fatal(err)
			}
			b := call(t)
			if b.State != tc.state || b.Deadline == nil || (*b.Deadline)[19:] != ".000Z" {
				t.Fatalf("%+v", b)
			}
		})
	}
}
