//go:build integration

package team_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/team"
)

func TestRemove(t *testing.T) {
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
	req := team.Request{Method: "DELETE", Route: "/ideas/{id}/members/{empNo}"}

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
	} {
		if err := s.Remove(ctx, id, tc.emp, tc.who, req); code(err) != tc.want {
			t.Errorf("%s: %v, want %s", name, err, tc.want)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member`); n != 2 {
		t.Errorf("refusals left %d rows, want 2", n)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE outcome = 'SUCCESS' AND action IN ('JOIN_REQUEST_REJECT','MEMBER_REMOVE')`); n != 0 {
		t.Errorf("refusals wrote %d SUCCESS rows", n)
	}

	// Reject a PENDING request.
	if err := s.Remove(ctx, id, "100002", leader, req); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member WHERE user_id = ?`, applicant.UserID.String()); n != 0 {
		t.Errorf("rejected row still there")
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_REJECT' AND outcome = 'SUCCESS'
	        AND target_id = ? AND status = 204`, id.String()); n != 1 {
		t.Errorf("JOIN_REQUEST_REJECT rows = %d, want 1", n)
	}
	// The rejected applicant may apply again at once.
	if _, err := s.Apply(ctx, applicant, "100001", "Idea A", req); err != nil {
		t.Errorf("re-apply after reject: %v", err)
	}

	// Remove an ACCEPTED Member.
	if err := s.Remove(ctx, id, "100003", leader, req); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'MEMBER_REMOVE' AND outcome = 'SUCCESS'
	        AND target_id = ? AND status = 204`, id.String()); n != 1 {
		t.Errorf("MEMBER_REMOVE rows = %d, want 1", n)
	}
	if err := s.Remove(ctx, id, "100003", leader, req); code(err) != web.CodeNotFound {
		t.Errorf("second remove: %v, want NOT_FOUND", err)
	}
}

func TestRemoveState(t *testing.T) {
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
		if err := s.Remove(context.Background(), id, "100002", leader, team.Request{}); code(err) != want {
			t.Errorf("%s: %v, want %s", expr, err, want)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member`); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestLeave(t *testing.T) {
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
	req := team.Request{Method: "DELETE", Route: "/ideas/{id}/members/me"}

	for name, who := range map[string]security.Principal{
		"stranger": stranger, "leader": leader, "admin": admin,
	} {
		if err := s.Leave(ctx, id, who, req); code(err) != web.CodeNotFound {
			t.Errorf("%s: %v, want NOT_FOUND", name, err)
		}
	}
	if err := s.Leave(ctx, uuid.New(), member, req); code(err) != web.CodeNotFound {
		t.Errorf("missing idea: %v, want NOT_FOUND", err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member`); n != 2 {
		t.Errorf("refusals left %d rows, want 2", n)
	}

	// Withdraw a PENDING request; the applicant may apply again at once.
	if err := s.Leave(ctx, id, applicant, req); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'JOIN_REQUEST_WITHDRAW' AND outcome = 'SUCCESS'
	        AND target_id = ? AND status = 204`, id.String()); n != 1 {
		t.Errorf("JOIN_REQUEST_WITHDRAW rows = %d, want 1", n)
	}
	if _, err := s.Apply(ctx, applicant, "100001", "Idea A", req); err != nil {
		t.Errorf("re-apply after withdraw: %v", err)
	}

	// Leave as an ACCEPTED Member.
	if err := s.Leave(ctx, id, member, req); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d, `SELECT COUNT(*) FROM audit_log WHERE action = 'MEMBER_LEAVE' AND outcome = 'SUCCESS'
	        AND target_id = ? AND status = 204`, id.String()); n != 1 {
		t.Errorf("MEMBER_LEAVE rows = %d, want 1", n)
	}
	if err := s.Leave(ctx, id, member, req); code(err) != web.CodeNotFound {
		t.Errorf("second leave: %v, want NOT_FOUND", err)
	}
}

func TestLeaveState(t *testing.T) {
	d := dbtest.New(t)
	leader := addUser(t, d, "100001", "USER")
	applicant := addUser(t, d, "100002", "USER")
	stranger := addUser(t, d, "100004", "USER")
	id := addIdea(t, d, leader.UserID, "Idea A")
	addRow(t, d, id, applicant.UserID, "PENDING")
	s := newService(d)
	for expr, want := range map[string]web.ErrorCode{
		"NULL":                    web.CodeSubmissionNotOpen,
		"NOW(6) - INTERVAL 1 DAY": web.CodeSubmissionClosed,
	} {
		setDeadline(t, d, expr)
		if err := s.Leave(context.Background(), id, applicant, team.Request{}); code(err) != want {
			t.Errorf("%s: %v, want %s", expr, err, want)
		}
		// A stranger never learns the Idea exists, whatever the window.
		if err := s.Leave(context.Background(), id, stranger, team.Request{}); code(err) != web.CodeNotFound {
			t.Errorf("%s stranger: %v, want NOT_FOUND", expr, err)
		}
	}
	if n := count(t, d, `SELECT COUNT(*) FROM idea_member`); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}
