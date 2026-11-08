// Package security holds the authenticated caller's identity: the Principal
// built by the session middleware and the context accessors every later layer
// uses to read it (Backend.md §6). It has no dependencies inside the module so
// that any package may import it.
package security

import "context"

// Role is an account role (PRD §3). Leader, Member and Applicant are relations
// to one Idea, not roles.
type Role string

// The two account roles.
const (
	RoleUser  Role = "USER"
	RoleAdmin Role = "ADMIN"
)

// Valid reports whether r is one of the two account roles.
func (r Role) Valid() bool { return r == RoleUser || r == RoleAdmin }

// ActorRole is the role recorded in the audit log (PRD §7): an account role, or
// ANONYMOUS (no session) or SYSTEM (no caller, e.g. background cleanup).
type ActorRole string

// The audit actor roles.
const (
	ActorUser      ActorRole = "USER"
	ActorAdmin     ActorRole = "ADMIN"
	ActorAnonymous ActorRole = "ANONYMOUS"
	ActorSystem    ActorRole = "SYSTEM"
)

// Principal is the caller resolved from a valid session. The role is read from
// the account on every request, never copied into the session (PRD §6.5).
type Principal struct {
	UserID int64
	EmpNo  string
	Role   Role
}

// IsAdmin reports whether the principal has role ADMIN.
func (p Principal) IsAdmin() bool { return p.Role == RoleAdmin }

// ActorRole maps the account role to its audit value. An invalid role maps to
// ANONYMOUS so an unexpected value is never recorded as privileged.
func (p Principal) ActorRole() ActorRole {
	switch p.Role {
	case RoleUser:
		return ActorUser
	case RoleAdmin:
		return ActorAdmin
	}
	return ActorAnonymous
}

type ctxKey int

const (
	principalKey ctxKey = iota
	systemKey
)

// WithPrincipal returns a context carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFrom returns the principal in ctx; ok is false when there is none.
func PrincipalFrom(ctx context.Context) (p Principal, ok bool) {
	p, ok = ctx.Value(principalKey).(Principal)
	return p, ok
}

// WithSystem marks ctx as a system action (no caller). A principal in ctx still
// wins, so a request can never be recorded as SYSTEM by accident.
func WithSystem(ctx context.Context) context.Context {
	return context.WithValue(ctx, systemKey, true)
}

// ActorRoleFrom returns the audit actor role for ctx: the principal's role,
// else SYSTEM if marked with WithSystem, else ANONYMOUS.
func ActorRoleFrom(ctx context.Context) ActorRole {
	if p, ok := PrincipalFrom(ctx); ok {
		return p.ActorRole()
	}
	if sys, _ := ctx.Value(systemKey).(bool); sys {
		return ActorSystem
	}
	return ActorAnonymous
}

// ActorEmpNo returns the caller's employee ID for the audit log, or "" when
// there is no principal.
func ActorEmpNo(ctx context.Context) string {
	p, _ := PrincipalFrom(ctx)
	return p.EmpNo
}
