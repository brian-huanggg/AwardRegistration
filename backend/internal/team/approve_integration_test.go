//go:build integration

package team_test

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/team"
)

func addRow(t *testing.T, d *sql.DB, idea, user uuid.UUID, status string) {
	t.Helper()
	accepted := "NULL"
	if status == "ACCEPTED" {
		accepted = "NOW(6)"
	}
	if _, err := d.Exec(`INSERT INTO idea_member (idea_id, user_id, status, accepted_at) VALUES (?,?,?,`+accepted+`)`,
		idea.String(), user.String(), status); err != nil {
		t.Fatal(err)
	}
}

func TestApprove(t *testing.T) {
	d := dbtest.New(t)
	setDeadline(t, d, "NOW(6) + INTERVAL 1 DAY")
	leader := addUser(t, d, "100001", "USER")
	applicant := addUser(t, d, "100002", "USER")
	member := addUser(t, d, "100003", "USER")
	stranger := addUser(t, d, "100004", "USER")
	admin := addUser(t, d, "900001", "ADMIN")
	id := addIdea(t, d, leader.UserID, "Idea A")
	addRow(t, d, id, applicant.UserID, "PENDING")
	addRow(t, d, id, member.UserID, "ACCEPTED")
	s := newService(d)
	ctx := context.Background()
	req := team.Request{Method: "POST", Route: "/ideas/{id}/members/{empNo}/approve"}

	for name, tc := range map[string]struct {
		who  security.Principal
		emp  string
		want web.ErrorCode
	}{
		"stranger":         {stranger, "100002", web.CodeNotFound},
		"applicant":        {applicant, "100002", web.CodeNotFound},
		"member":           {member, "100002", web.CodeForbidden},
		"admin":            {admin, "100002", web.CodeForbidden},
		"no row":           {leader, "100004", web.CodeNotFound},
		"leader own empNo": {leader, "100001", web.CodeNotFound},
		"bad empNo":        {leader, "x", web.CodeNotFound},
		"already member":   {leader, "100003", web.CodeJoinRequestNotPend},
	} {
		if err := s.Approve(ctx, id, tc.emp, tc.who, req); code(err) != tc.want {
			t.Errorf("%s: %v, want %s", name, err, tc.want)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_APPROVE'`); n != 0 {
		t.Errorf("refusals wrote %d SUCCESS rows", n)
	}

	if err := s.Approve(ctx, id, "100002", leader, req); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member WHERE user_id = ? AND status = 'ACCEPTED' AND accepted_at IS NOT NULL`,
		applicant.UserID.String()); n != 1 {
		t.Errorf("applicant not accepted")
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_APPROVE' AND outcome = 'SUCCESS'
	        AND target_id = ? AND status = 200`, id.String()); n != 1 {
		t.Errorf("JOIN_REQUEST_APPROVE rows = %d, want 1", n)
	}
	if err := s.Approve(ctx, id, "100002", leader, req); code(err) != web.CodeJoinRequestNotPend {
		t.Errorf("second approve: %v", err)
	}
}

func TestApproveTeamFull(t *testing.T) {
	d := dbtest.New(t)
	setDeadline(t, d, "NOW(6) + INTERVAL 1 DAY")
	leader := addUser(t, d, "100001", "USER")
	id := addIdea(t, d, leader.UserID, "Idea A")
	for _, e := range []string{"100002", "100003", "100004"} {
		addRow(t, d, id, addUser(t, d, e, "USER").UserID, "ACCEPTED")
	}
	addRow(t, d, id, addUser(t, d, "100005", "USER").UserID, "PENDING")
	s := newService(d)
	if err := s.Approve(context.Background(), id, "100005", leader, team.Request{}); code(err) != web.CodeTeamFull {
		t.Errorf("got %v, want TEAM_FULL", err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member WHERE status = 'PENDING'`); n != 1 {
		t.Errorf("pending rows = %d, want 1", n)
	}
}

func TestApproveRoleAndState(t *testing.T) {
	d := dbtest.New(t)
	leader := addUser(t, d, "100001", "USER")
	id := addIdea(t, d, leader.UserID, "Idea A")
	addRow(t, d, id, addUser(t, d, "100002", "USER").UserID, "PENDING")
	s := newService(d)
	for expr, want := range map[string]web.ErrorCode{
		"NULL":                    web.CodeSubmissionNotOpen,
		"NOW(6) - INTERVAL 1 DAY": web.CodeSubmissionClosed,
	} {
		setDeadline(t, d, expr)
		if err := s.Approve(context.Background(), id, "100002", leader, team.Request{}); code(err) != want {
			t.Errorf("%s: %v, want %s", expr, err, want)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member WHERE status = 'ACCEPTED'`); n != 0 {
		t.Errorf("accepted = %d, want 0", n)
	}
}

// Many pending requests approved at once never exceed 3 approved Members.
func TestApproveConcurrentNeverExceedsThree(t *testing.T) {
	d := dbtest.New(t)
	setDeadline(t, d, "NOW(6) + INTERVAL 1 DAY")
	leader := addUser(t, d, "100001", "USER")
	id := addIdea(t, d, leader.UserID, "Idea A")
	const n = 8
	emps := make([]string, n)
	for i := range n {
		emps[i] = "20000" + string(rune('0'+i))
		addRow(t, d, id, addUser(t, d, emps[i], "USER").UserID, "PENDING")
	}
	s := newService(d)

	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { errs[i] = s.Approve(context.Background(), id, emps[i], leader, team.Request{}) })
	}
	wg.Wait()
	ok, full := 0, 0
	for _, err := range errs {
		switch code(err) {
		case "":
			ok++
		case web.CodeTeamFull:
			full++
		default:
			t.Errorf("unexpected: %v", err)
		}
	}
	if ok != 3 || full != n-3 {
		t.Errorf("ok %d full %d, want 3 and %d", ok, full, n-3)
	}
	if c := count(t, d, `SELECT COUNT(*) FROM idea_member WHERE status = 'ACCEPTED'`); c != 3 {
		t.Errorf("accepted = %d, want 3", c)
	}
	if c := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_APPROVE'`); c != 3 {
		t.Errorf("audit rows = %d, want 3", c)
	}
}
