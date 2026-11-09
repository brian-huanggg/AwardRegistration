package deadline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func TestGatePrecheck(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	user := security.Principal{Role: security.RoleUser}
	admin := security.Principal{Role: security.RoleAdmin}

	tests := []struct {
		name  string
		d     *time.Time
		actor security.Principal
		want  web.ErrorCode
	}{
		{"user NOT_OPEN", nil, user, web.CodeSubmissionNotOpen},
		{"user OPEN", &future, user, ""},
		{"user CLOSED", &past, user, web.CodeSubmissionClosed},
		{"user at the deadline instant is CLOSED", &now, user, web.CodeSubmissionClosed},
		{"admin NOT_OPEN", nil, admin, ""},
		{"admin CLOSED", &past, admin, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := deadline.Gate{Store: fakeStore{snap: deadline.Snapshot{Deadline: tc.d, Now: now}}}
			err := g.Precheck(context.Background(), tc.actor)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("got %v", err)
				}
				return
			}
			var e *web.Error
			if !errors.As(err, &e) || e.Code != tc.want || e.Status != 403 {
				t.Fatalf("got %v, want %s/403", err, tc.want)
			}
		})
	}
}

func TestGatePrecheckStoreError(t *testing.T) {
	boom := errors.New("db down")
	g := deadline.Gate{Store: fakeStore{err: boom}}
	if err := g.Precheck(context.Background(), security.Principal{Role: security.RoleUser}); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	// Admin is exempt, so the store is not even read.
	if err := g.Precheck(context.Background(), security.Principal{Role: security.RoleAdmin}); err != nil {
		t.Fatalf("admin: %v", err)
	}
}
