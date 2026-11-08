// Package audit writes the Audit Log (PRD §6.11, Backend.md §7). Two tracks:
//
//   - Record joins the caller's transaction, so a successful write and its audit
//     row commit or roll back together (fail-closed).
//   - RecordRejected opens its own transaction for DENIED / FAILED / THROTTLED
//     rows; it is best effort and counts its failures.
//
// The table is append-only: this package only ever INSERTs.
package audit

import (
	"encoding/json"
	"net/netip"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

// Action is the audit action (PRD §6.11).
type Action string

// The recorded actions.
const (
	AuthRegister        Action = "AUTH_REGISTER"
	AuthLogin           Action = "AUTH_LOGIN"
	AuthLogout          Action = "AUTH_LOGOUT"
	IdeaCreate          Action = "IDEA_CREATE"
	IdeaUpdate          Action = "IDEA_UPDATE"
	IdeaDelete          Action = "IDEA_DELETE"
	IdeaView            Action = "IDEA_VIEW"
	SlidesReplace       Action = "SLIDES_REPLACE"
	SlidesDownload      Action = "SLIDES_DOWNLOAD"
	JoinRequestCreate   Action = "JOIN_REQUEST_CREATE"
	JoinRequestApprove  Action = "JOIN_REQUEST_APPROVE"
	JoinRequestReject   Action = "JOIN_REQUEST_REJECT"
	JoinRequestWithdraw Action = "JOIN_REQUEST_WITHDRAW"
	MemberRemove        Action = "MEMBER_REMOVE"
	MemberLeave         Action = "MEMBER_LEAVE"
	DeadlineUpdate      Action = "DEADLINE_UPDATE"
	AdminSync           Action = "ADMIN_SYNC"
	AuditQuery          Action = "AUDIT_QUERY"
)

var actions = map[Action]bool{
	AuthRegister: true, AuthLogin: true, AuthLogout: true,
	IdeaCreate: true, IdeaUpdate: true, IdeaDelete: true, IdeaView: true,
	SlidesReplace: true, SlidesDownload: true,
	JoinRequestCreate: true, JoinRequestApprove: true, JoinRequestReject: true, JoinRequestWithdraw: true,
	MemberRemove: true, MemberLeave: true,
	DeadlineUpdate: true, AdminSync: true, AuditQuery: true,
}

// Valid reports whether a is a known action.
func (a Action) Valid() bool { return actions[a] }

// Outcome is the result of the audited operation.
type Outcome string

// The outcomes. SUCCESS goes through Record; the other three are rejected
// attempts and go through RecordRejected (ADMIN_SYNC FAILED may use either).
const (
	Success   Outcome = "SUCCESS"
	Failed    Outcome = "FAILED"
	Denied    Outcome = "DENIED"
	Throttled Outcome = "THROTTLED"
)

// Valid reports whether o is a known outcome.
func (o Outcome) Valid() bool {
	return o == Success || o == Failed || o == Denied || o == Throttled
}

// TargetType is the kind of resource a row is about.
type TargetType string

// The target types. DEADLINE has no ID.
const (
	TargetUser     TargetType = "USER"
	TargetIdea     TargetType = "IDEA"
	TargetDeadline TargetType = "DEADLINE"
)

// Valid reports whether t is a known target type.
func (t TargetType) Valid() bool {
	return t == TargetUser || t == TargetIdea || t == TargetDeadline
}

// Event is one audit row. The request ID and, unless overridden, the actor come
// from the context; occurred_at is the database clock.
type Event struct {
	Action  Action
	Outcome Outcome

	// Actor override. Needed where the context has no principal yet (a login
	// that just succeeded) or the actor is not the caller (ADMIN_SYNC). When
	// ActorRole is empty both fields come from the context.
	ActorRole  security.ActorRole
	ActorEmpNo string

	TargetType TargetType
	TargetID   uuid.UUID // uuid.Nil = none

	Method    string
	Route     string // route template, e.g. /ideas/{id}
	Status    int    // HTTP status; 0 = none
	ErrorCode string
	IP        netip.Addr // zero = none
	UserAgent string

	// SubmittedEmpNo is the raw employee ID typed into a failed login or
	// registration; it is sanitized before it is stored.
	SubmittedEmpNo string

	// Before and After are JSON documents (changed fields only). Secrets are
	// redacted by key name before they are stored.
	Before json.RawMessage
	After  json.RawMessage
}
