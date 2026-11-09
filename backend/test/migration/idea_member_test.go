//go:build integration

package migration_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

func mkUser(t *testing.T, d *sql.DB, empNo string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := d.Exec(`INSERT INTO app_user (id, emp_no, name, email, password_hash, role) VALUES (?,?,?,?,?,'USER')`,
		id, empNo, "U"+empNo, empNo+"@x", hash); err != nil {
		t.Fatal(err)
	}
	return id
}

func mkIdea(t *testing.T, d *sql.DB, leader, name string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := d.Exec(`INSERT INTO idea (id, leader_id, category, org, direction, name, summary,
	        slides_object_key, slides_ext, slides_size, slides_sha256)
	        VALUES (?,?,'COOL_IDEA','BSID','TALENT_DEVELOPMENT',?,'s',?,'pdf',1024,?)`,
		id, leader, name, "slides/"+id+"/k.pdf", strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestIdeaMemberRules(t *testing.T) {
	d := dbtest.New(t)
	leader := mkUser(t, d, "100001")
	u1, u2 := mkUser(t, d, "100002"), mkUser(t, d, "100003")
	idea := mkIdea(t, d, leader, "I1")
	other := mkIdea(t, d, leader, "I2")

	exec := func(q string, args ...any) error { _, err := d.Exec(q, args...); return err }

	if err := exec(`INSERT INTO idea_member (idea_id, user_id) VALUES (?,?)`, idea, u1); err != nil {
		t.Fatalf("default status PENDING: %v", err)
	}
	var st string
	if err := d.QueryRow(`SELECT status FROM idea_member WHERE idea_id=? AND user_id=?`, idea, u1).Scan(&st); err != nil || st != "PENDING" {
		t.Fatalf("status = %q, %v", st, err)
	}
	if err := exec(`INSERT INTO idea_member (idea_id, user_id, status, accepted_at) VALUES (?,?,'ACCEPTED',NOW(6))`, idea, u2); err != nil {
		t.Errorf("ACCEPTED with accepted_at: %v", err)
	}
	if err := exec(`INSERT INTO idea_member (idea_id, user_id) VALUES (?,?)`, other, u1); err != nil {
		t.Errorf("same User on a second Idea: %v", err)
	}

	for name, err := range map[string]error{
		"duplicate (idea,user)":       exec(`INSERT INTO idea_member (idea_id, user_id) VALUES (?,?)`, idea, u1),
		"unknown status":              exec(`INSERT INTO idea_member (idea_id, user_id, status) VALUES (?,?,'REJECTED')`, other, u2),
		"lower-case status":           exec(`INSERT INTO idea_member (idea_id, user_id, status) VALUES (?,?,'pending')`, other, u2),
		"ACCEPTED without time":       exec(`INSERT INTO idea_member (idea_id, user_id, status) VALUES (?,?,'ACCEPTED')`, other, u2),
		"PENDING with accepted_at":    exec(`INSERT INTO idea_member (idea_id, user_id, accepted_at) VALUES (?,?,NOW(6))`, other, u2),
		"unknown idea":                exec(`INSERT INTO idea_member (idea_id, user_id) VALUES (?,?)`, uuid.NewString(), u2),
		"unknown user":                exec(`INSERT INTO idea_member (idea_id, user_id) VALUES (?,?)`, other, uuid.NewString()),
		"approve without accepted_at": exec(`UPDATE idea_member SET status='ACCEPTED' WHERE idea_id=? AND user_id=?`, idea, u1),
	} {
		if err == nil {
			t.Errorf("%s: want rejected", name)
		}
	}

	// Approve sets both columns together.
	if err := exec(`UPDATE idea_member SET status='ACCEPTED', accepted_at=NOW(6) WHERE idea_id=? AND user_id=?`, idea, u1); err != nil {
		t.Errorf("approve: %v", err)
	}

	// Deleting an Idea removes its rows only.
	if err := exec(`DELETE FROM idea WHERE id=?`, idea); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM idea_member`).Scan(&n); err != nil || n != 1 {
		t.Errorf("rows after Idea delete = %d (want 1, the other Idea's), %v", n, err)
	}
	// A User with memberships cannot be deleted.
	if err := exec(`DELETE FROM app_user WHERE id=?`, u1); err == nil {
		t.Error("deleting a User with a membership: want rejected")
	}
}
