//go:build integration

package team_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/team"
)

const hash = "$2a$10$" + "abcdefghijklmnopqrstuuabcdefghijklmnopqrstuuabcdefghi"

func addUser(t *testing.T, d *sql.DB, empNo, role string) security.Principal {
	t.Helper()
	id := uuid.New()
	var email any
	if role == "USER" {
		email = empNo + "@example.com"
	}
	if _, err := d.Exec(`INSERT INTO app_user (id, emp_no, name, email, password_hash, role) VALUES (?,?,?,?,?,?)`,
		id.String(), empNo, "User "+empNo, email, hash, role); err != nil {
		t.Fatal(err)
	}
	return security.Principal{UserID: id, EmpNo: empNo, Role: security.Role(role)}
}

func addIdea(t *testing.T, d *sql.DB, leader uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := d.Exec(`INSERT INTO idea (id, leader_id, category, org, direction, name, summary,
	        slides_object_key, slides_ext, slides_size, slides_sha256)
	        VALUES (?, ?, 'COOL_IDEA', 'BSID', 'TALENT_DEVELOPMENT', ?, 's', ?, 'pdf', 1024, ?)`,
		id.String(), leader.String(), name, "slides/"+id.String()+"/"+uuid.NewString()+".pdf",
		strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}
	return id
}

func setDeadline(t *testing.T, d *sql.DB, expr string) {
	t.Helper()
	if _, err := d.Exec(`UPDATE contest_setting SET deadline = ` + expr + ` WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
}

func newService(d *sql.DB) *team.Service {
	gate := deadline.Gate{Store: deadline.SQLStore{DB: d}}
	return &team.Service{DB: d, Audit: audit.New(d, nil), Gate: gate,
		Ideas: &idea.Service{DB: d, Gate: gate}}
}

func code(err error) web.ErrorCode {
	var e *web.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func count(t *testing.T, d *sql.DB, q string, args ...any) (n int) {
	t.Helper()
	if err := d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestApply(t *testing.T) {
	d := dbtest.New(t)
	setDeadline(t, d, "NOW(6) + INTERVAL 1 DAY")
	leader := addUser(t, d, "100001", "USER")
	applicant := addUser(t, d, "100002", "USER")
	member := addUser(t, d, "100003", "USER")
	ideaID := addIdea(t, d, leader.UserID, "Idea A")
	if _, err := d.Exec(`INSERT INTO idea_member (idea_id, user_id, status, accepted_at) VALUES (?,?,'ACCEPTED',NOW(6))`,
		ideaID.String(), member.UserID.String()); err != nil {
		t.Fatal(err)
	}
	s := newService(d)
	ctx := context.Background()
	req := team.Request{Method: "POST", Route: "/join-requests"}

	// Name is cleaned before the lookup; a full team may still be applied to.
	got, err := s.Apply(ctx, applicant, "100001", "  Idea \t A ", req)
	if err != nil {
		t.Fatal(err)
	}
	if got.IdeaID != ideaID || got.IdeaName != "Idea A" || got.LeaderEmpNo != "100001" ||
		got.LeaderName != "User 100001" || got.Category != "COOL_IDEA" || got.RequestedAt.IsZero() {
		t.Errorf("got %+v", got)
	}
	var status string
	if err := d.QueryRow(`SELECT status FROM idea_member WHERE idea_id = ? AND user_id = ?`,
		ideaID.String(), applicant.UserID.String()).Scan(&status); err != nil || status != "PENDING" {
		t.Errorf("row status %q, %v", status, err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_CREATE' AND outcome = 'SUCCESS'
	        AND target_id = ? AND status = 201`, ideaID.String()); n != 1 {
		t.Errorf("JOIN_REQUEST_CREATE rows = %d, want 1", n)
	}

	for name, tc := range map[string]struct {
		who       security.Principal
		emp, idea string
		want      web.ErrorCode
	}{
		"pending again":  {applicant, "100001", "Idea A", web.CodeJoinRequestExists},
		"already member": {member, "100001", "Idea A", web.CodeAlreadyMember},
		"own idea":       {leader, "100001", "Idea A", web.CodeCannotJoinOwnIdea},
		"unknown name":   {applicant, "100001", "idea a", web.CodeNotFound}, // case-sensitive
		"unknown leader": {applicant, "999999", "Idea A", web.CodeNotFound},
		"bad emp no":     {applicant, "1", "Idea A", web.CodeValidationFailed},
		"empty name":     {applicant, "100001", "  ", web.CodeValidationFailed},
	} {
		if _, err := s.Apply(ctx, tc.who, tc.emp, tc.idea, req); code(err) != tc.want {
			t.Errorf("%s: %v, want %s", name, err, tc.want)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_CREATE'`); n != 1 {
		t.Errorf("failed applies wrote %d extra success rows", n-1)
	}
}

// State is checked again inside the transaction; Admin is refused by role.
func TestApplyRoleAndState(t *testing.T) {
	d := dbtest.New(t)
	leader := addUser(t, d, "100001", "USER")
	applicant := addUser(t, d, "100002", "USER")
	admin := addUser(t, d, "900001", "ADMIN")
	addIdea(t, d, leader.UserID, "Idea A")
	s := newService(d)
	ctx := context.Background()

	if err := s.AuthorizeApply(ctx, admin); code(err) != web.CodeForbidden {
		t.Errorf("admin: %v", err)
	}
	for expr, want := range map[string]web.ErrorCode{
		"NULL":                     web.CodeSubmissionNotOpen,
		"NOW(6) - INTERVAL 1 DAY":  web.CodeSubmissionClosed,
		"NOW(6) + INTERVAL 1 HOUR": "",
	} {
		setDeadline(t, d, expr)
		if err := s.AuthorizeApply(ctx, applicant); code(err) != want {
			t.Errorf("precheck %s: %v, want %q", expr, err, want)
		}
	}
	// Race: the pre-check passed, then the deadline passed.
	setDeadline(t, d, "NOW(6) - INTERVAL 1 DAY")
	if _, err := s.Apply(ctx, applicant, "100001", "Idea A", team.Request{}); code(err) != web.CodeSubmissionClosed {
		t.Errorf("apply after close: %v", err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member`); n != 0 {
		t.Errorf("rows = %d, want 0", n)
	}
}

// Concurrent identical applies: exactly one row and one audit row; the rest
// answer JOIN_REQUEST_EXISTS.
func TestApplyConcurrent(t *testing.T) {
	d := dbtest.New(t)
	setDeadline(t, d, "NOW(6) + INTERVAL 1 DAY")
	leader := addUser(t, d, "100001", "USER")
	applicant := addUser(t, d, "100002", "USER")
	addIdea(t, d, leader.UserID, "Idea A")
	s := newService(d)

	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { _, errs[i] = s.Apply(context.Background(), applicant, "100001", "Idea A", team.Request{}) })
	}
	wg.Wait()
	ok, exists := 0, 0
	for _, err := range errs {
		switch code(err) {
		case "":
			ok++
		case web.CodeJoinRequestExists:
			exists++
		default:
			t.Errorf("unexpected: %v", err)
		}
	}
	if ok != 1 || exists != n-1 {
		t.Errorf("ok %d exists %d", ok, exists)
	}
	if c := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_CREATE'`); c != 1 {
		t.Errorf("audit rows = %d, want 1", c)
	}
}

// A request for an Idea deleted after the lookup is the uniform 404.
func TestApplyIdeaDeletedAfterLookup(t *testing.T) {
	d := dbtest.New(t)
	setDeadline(t, d, "NOW(6) + INTERVAL 1 DAY")
	leader := addUser(t, d, "100001", "USER")
	applicant := addUser(t, d, "100002", "USER")
	id := addIdea(t, d, leader.UserID, "Idea A")
	s := newService(d)
	s.Ideas = stale{real: s.Ideas, onFind: func() {
		if _, err := d.Exec(`DELETE FROM idea WHERE id = ?`, id.String()); err != nil {
			t.Error(err)
		}
	}}
	if _, err := s.Apply(context.Background(), applicant, "100001", "Idea A", team.Request{}); code(err) != web.CodeNotFound {
		t.Errorf("got %v, want NOT_FOUND", err)
	}
}

// ListMine returns only the caller's PENDING rows, newest first, with slim Idea
// info; Admin gets an empty (non-nil) list.
func TestListMine(t *testing.T) {
	d := dbtest.New(t)
	setDeadline(t, d, "NOW(6) + INTERVAL 1 DAY")
	leader := addUser(t, d, "100001", "USER")
	applicant := addUser(t, d, "100002", "USER")
	other := addUser(t, d, "100003", "USER")
	admin := addUser(t, d, "900001", "ADMIN")
	a := addIdea(t, d, leader.UserID, "Idea A")
	b := addIdea(t, d, leader.UserID, "Idea B")
	c := addIdea(t, d, leader.UserID, "Idea C")
	for _, r := range []struct {
		idea   uuid.UUID
		user   security.Principal
		status string
		at     string
	}{
		{a, applicant, "PENDING", "NOW(6) - INTERVAL 2 HOUR"},
		{b, applicant, "PENDING", "NOW(6) - INTERVAL 1 HOUR"},
		{c, applicant, "ACCEPTED", "NOW(6)"}, // Member, not a request
		{a, other, "PENDING", "NOW(6)"},      // someone else's
	} {
		accepted := "NULL"
		if r.status == "ACCEPTED" {
			accepted = "NOW(6)"
		}
		if _, err := d.Exec(`INSERT INTO idea_member (idea_id, user_id, status, created_at, accepted_at) VALUES (?,?,?,`+r.at+`,`+accepted+`)`,
			r.idea.String(), r.user.UserID.String(), r.status); err != nil {
			t.Fatal(err)
		}
	}
	s := newService(d)
	ctx := context.Background()

	got, err := s.ListMine(ctx, applicant)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].IdeaID != b || got[1].IdeaID != a {
		t.Fatalf("got %+v, want [B, A]", got)
	}
	if g := got[0]; g.IdeaName != "Idea B" || g.Category != "COOL_IDEA" || g.LeaderEmpNo != "100001" ||
		g.LeaderName != "User 100001" || g.RequestedAt.IsZero() {
		t.Errorf("slim info: %+v", g)
	}

	if got, err := s.ListMine(ctx, leader); err != nil || len(got) != 0 {
		t.Errorf("leader: %v, %v", got, err)
	}
	if got, err := s.ListMine(ctx, admin); err != nil || got == nil || len(got) != 0 {
		t.Errorf("admin: %#v, %v", got, err)
	}
}

type stale struct {
	real   team.Ideas
	onFind func()
}

func (s stale) FindByLeader(ctx context.Context, emp, name string) (idea.Idea, error) {
	got, err := s.real.FindByLeader(ctx, emp, name)
	s.onFind()
	return got, err
}
