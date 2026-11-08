package security_test

import (
	"context"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

func TestPrincipalRoundTrip(t *testing.T) {
	ctx := context.Background()
	if _, ok := security.PrincipalFrom(ctx); ok {
		t.Fatal("empty context must have no principal")
	}
	want := security.Principal{UserID: 7, EmpNo: "A123", Role: security.RoleUser}
	got, ok := security.PrincipalFrom(security.WithPrincipal(ctx, want))
	if !ok || got != want {
		t.Fatalf("got %+v, %v", got, ok)
	}
}

func TestActorRole(t *testing.T) {
	bg := context.Background()
	user := security.Principal{UserID: 1, EmpNo: "U1", Role: security.RoleUser}
	admin := security.Principal{UserID: 2, EmpNo: "A1", Role: security.RoleAdmin}
	bad := security.Principal{UserID: 3, EmpNo: "X", Role: "ROOT"}

	tests := []struct {
		name string
		ctx  context.Context
		want security.ActorRole
		emp  string
	}{
		{"anonymous", bg, security.ActorAnonymous, ""},
		{"user", security.WithPrincipal(bg, user), security.ActorUser, "U1"},
		{"admin", security.WithPrincipal(bg, admin), security.ActorAdmin, "A1"},
		{"invalid role is never privileged", security.WithPrincipal(bg, bad), security.ActorAnonymous, "X"},
		{"system", security.WithSystem(bg), security.ActorSystem, ""},
		{"principal wins over system", security.WithSystem(security.WithPrincipal(bg, user)), security.ActorUser, "U1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := security.ActorRoleFrom(tc.ctx); got != tc.want {
				t.Errorf("ActorRoleFrom = %q, want %q", got, tc.want)
			}
			if got := security.ActorEmpNo(tc.ctx); got != tc.emp {
				t.Errorf("ActorEmpNo = %q, want %q", got, tc.emp)
			}
		})
	}
}

func TestRole(t *testing.T) {
	if !security.RoleUser.Valid() || !security.RoleAdmin.Valid() || security.Role("").Valid() || security.Role("ROOT").Valid() {
		t.Error("Valid wrong")
	}
	if !(security.Principal{Role: security.RoleAdmin}).IsAdmin() || (security.Principal{Role: security.RoleUser}).IsAdmin() {
		t.Error("IsAdmin wrong")
	}
}
