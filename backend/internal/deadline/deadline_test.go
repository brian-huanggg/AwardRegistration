package deadline_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
)

type fakeStore struct {
	snap deadline.Snapshot
	err  error
}

func (f fakeStore) Snapshot(context.Context) (deadline.Snapshot, error) { return f.snap, f.err }

func get(t *testing.T, st deadline.Store) *httptest.ResponseRecorder {
	t.Helper()
	h := server.NewPublic(slog.New(slog.DiscardHandler), server.Mount(server.API{Deadline: deadline.Handler{Store: st}}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/deadline", http.NoBody))
	return rec
}

func TestStateOf(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	before, after := now.Add(time.Second), now.Add(-time.Second)
	tests := []struct {
		name string
		d    *time.Time
		want api.DeadlineState
	}{
		{"no deadline", nil, api.NOTOPEN},
		{"before deadline", &before, api.OPEN},
		{"exactly at deadline", &now, api.CLOSED},
		{"after deadline", &after, api.CLOSED},
	}
	for _, tc := range tests {
		if got := deadline.StateOf(now, tc.d); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestGetDeadline(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 999_999_000, time.UTC)
	d := now.Add(time.Hour).Truncate(time.Second)
	tests := []struct {
		name string
		snap deadline.Snapshot
		want string
	}{
		{"not open", deadline.Snapshot{Now: now},
			`{"deadline":null,"state":"NOT_OPEN","serverTime":"2026-11-08T12:00:00.999Z"}`},
		{"open", deadline.Snapshot{Deadline: &d, Now: now},
			`{"deadline":"2026-11-08T13:00:00.000Z","state":"OPEN","serverTime":"2026-11-08T12:00:00.999Z"}`},
	}
	for _, tc := range tests {
		rec := get(t, fakeStore{snap: tc.snap})
		if rec.Code != 200 || rec.Body.String() != tc.want+"\n" {
			t.Errorf("%s: %d %q", tc.name, rec.Code, rec.Body)
		}
	}
}

func TestGetDeadlineStoreFailureIs503(t *testing.T) {
	rec := get(t, fakeStore{err: errors.New("connection refused")})
	if rec.Code != 503 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	if b := rec.Body.String(); !strings.Contains(b, `"code":"SERVICE_UNAVAILABLE"`) || strings.Contains(b, "refused") {
		t.Fatalf("body %s", b)
	}
}
