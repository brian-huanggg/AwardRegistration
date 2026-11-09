//go:build integration

package idea_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

const hash = "$2a$10$" + "abcdefghijklmnopqrstuuabcdefghijklmnopqrstuuabcdefghi" // 7 + 53 chars

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

func newIdea(leader uuid.UUID, name string) store.NewIdea {
	id := uuid.New()
	return store.NewIdea{
		ID: id, LeaderID: leader, Category: "COOL_IDEA", Org: "BSID", Direction: "TALENT_DEVELOPMENT",
		Name: name, Summary: "s",
		Slides: store.Slides{ObjectKey: "slides/" + id.String() + "/" + uuid.NewString() + ".pdf",
			Ext: "pdf", Size: 1024, SHA256: strings.Repeat("ab", 32)},
	}
}

func insert(d *sql.DB, n store.NewIdea) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	if err := store.InsertIdea(context.Background(), tx, n); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func TestVisibilityPerRole(t *testing.T) {
	d := dbtest.New(t)
	leader := addUser(t, d, "100001", "USER")
	other := addUser(t, d, "100002", "USER")
	admin := addUser(t, d, "900001", "ADMIN")
	n := newIdea(leader.UserID, "Idea A")
	if err := insert(d, n); err != nil {
		t.Fatal(err)
	}
	svc := &idea.Service{DB: d}
	ctx := context.Background()

	for _, tc := range []struct {
		who     string
		p       security.Principal
		visible bool
	}{{"leader", leader, true}, {"admin", admin, true}, {"other user", other, false}} {
		got, err := svc.Get(ctx, n.ID, tc.p)
		if tc.visible {
			if err != nil || got.ID != n.ID || got.LeaderEmpNo != "100001" || got.Version != 0 || got.Slides.Ext != "pdf" {
				t.Errorf("%s: got %+v, %v", tc.who, got, err)
			}
			continue
		}
		if !isNotFound(err) {
			t.Errorf("%s: want NOT_FOUND, got %v", tc.who, err)
		}
	}

	// A missing Idea is indistinguishable from a hidden one.
	_, errMissing := svc.Get(ctx, uuid.New(), leader)
	_, errHidden := svc.Get(ctx, n.ID, other)
	if !isNotFound(errMissing) || errMissing.Error() != errHidden.Error() {
		t.Errorf("missing %v vs hidden %v", errMissing, errHidden)
	}
}

func isNotFound(err error) bool {
	var we *web.Error
	return errors.As(err, &we) && we.Error() == "NOT_FOUND"
}

func TestUniqueLeaderName(t *testing.T) {
	d := dbtest.New(t)
	a := addUser(t, d, "100001", "USER")
	b := addUser(t, d, "100002", "USER")
	if err := insert(d, newIdea(a.UserID, "Same")); err != nil {
		t.Fatal(err)
	}
	err := insert(d, newIdea(a.UserID, "Same"))
	if key, ok := db.DuplicateKey(err); !ok || key != "uq_idea_leader_name" {
		t.Errorf("same leader, same name: key %q ok %v err %v", key, ok, err)
	}
	if err := insert(d, newIdea(a.UserID, "same")); err != nil {
		t.Errorf("name is case-sensitive: %v", err)
	}
	if err := insert(d, newIdea(b.UserID, "Same")); err != nil {
		t.Errorf("names are unique per leader, not globally: %v", err)
	}
}

func TestChecksRejectBadRows(t *testing.T) {
	d := dbtest.New(t)
	a := addUser(t, d, "100001", "USER")
	for name, mut := range map[string]func(*store.NewIdea){
		"category":       func(n *store.NewIdea) { n.Category = "X" },
		"org":            func(n *store.NewIdea) { n.Org = "ZZZZ" },
		"direction":      func(n *store.NewIdea) { n.Direction = "X" },
		"empty name":     func(n *store.NewIdea) { n.Name = "" },
		"slash name":     func(n *store.NewIdea) { n.Name = "a/b" },
		"long name":      func(n *store.NewIdea) { n.Name = strings.Repeat("好", 76) },
		"empty summary":  func(n *store.NewIdea) { n.Summary = "" },
		"ext":            func(n *store.NewIdea) { n.Slides.Ext = "docx" },
		"size zero":      func(n *store.NewIdea) { n.Slides.Size = 0 },
		"size over 5MiB": func(n *store.NewIdea) { n.Slides.Size = 5*1024*1024 + 1 },
		"sha upper":      func(n *store.NewIdea) { n.Slides.SHA256 = strings.Repeat("AB", 32) },
		"no leader":      func(n *store.NewIdea) { n.LeaderID = uuid.New() },
	} {
		n := newIdea(a.UserID, "ok")
		mut(&n)
		if err := insert(d, n); err == nil {
			t.Errorf("%s: row accepted", name)
		}
	}
	n := newIdea(a.UserID, strings.Repeat("好", 75))
	n.Slides.Size = 5 * 1024 * 1024
	if err := insert(d, n); err != nil {
		t.Errorf("boundary values rejected: %v", err)
	}
}
