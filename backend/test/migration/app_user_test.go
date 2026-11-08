//go:build integration

// Package migration holds schema-level tests against a real MariaDB with the real migrations.
package migration_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

const hash = "$2a$10$" + "abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234" // 53 chars after the prefix

func insert(d *sql.DB, empNo, name string, email any, pw, role string) error {
	_, err := d.Exec(`INSERT INTO app_user (emp_no, name, email, password_hash, role) VALUES (?,?,?,?,?)`,
		empNo, name, email, pw, role)
	return err
}

func TestAppUserFieldRules(t *testing.T) {
	d := dbtest.New(t)
	if len(hash) != 60 {
		t.Fatalf("test hash has length %d", len(hash))
	}

	valid := []struct {
		name, empNo, nm string
		email           any
		role            string
	}{
		{"user", "089290", "王小明", "a@b", "USER"},
		{"user 50 code points", "000001", strings.Repeat("名", 50), "x@y.z", "USER"},
		{"admin without email", "000002", "Admin", nil, "ADMIN"},
		{"second admin (NULL emails do not collide)", "000003", "Admin 2", nil, "ADMIN"},
		{"email max length", "000004", "N", strings.Repeat("a", 252) + "@b", "USER"},
	}
	for _, c := range valid {
		if err := insert(d, c.empNo, c.nm, c.email, hash, c.role); err != nil {
			t.Errorf("%s: want accepted, got %v", c.name, err)
		}
	}

	invalid := []struct {
		name, empNo, nm string
		email           any
		pw, role        string
	}{
		{"emp_no 5 digits", "12345", "N", "e1@x", hash, "USER"},
		{"emp_no 7 digits", "1234567", "N", "e2@x", hash, "USER"},
		{"emp_no letter", "12345a", "N", "e3@x", hash, "USER"},
		{"emp_no newline", "12345\n", "N", "e4@x", hash, "USER"},
		{"emp_no fullwidth digit", "12345１", "N", "e5@x", hash, "USER"},
		{"name empty", "100001", "", "e6@x", hash, "USER"},
		{"name 51 code points", "100002", strings.Repeat("名", 51), "e7@x", hash, "USER"},
		{"name control char", "100003", "a\tb", "e8@x", hash, "USER"},
		{"name slash", "100004", "a/b", "e9@x", hash, "USER"},
		{"name backslash", "100005", `a\b`, "e10@x", hash, "USER"},
		{"name pipe", "100006", "a|b", "e11@x", hash, "USER"},
		{"name quote", "100007", `a"b`, "e12@x", hash, "USER"},
		{"email upper case", "100008", "N", "A@x", hash, "USER"},
		{"email space", "100009", "N", "a b@x", hash, "USER"},
		{"email non-ASCII", "100010", "N", "é@x", hash, "USER"},
		{"email two @", "100011", "N", "a@b@c", hash, "USER"},
		{"email no @", "100012", "N", "abc", hash, "USER"},
		{"email empty local", "100013", "N", "@x", hash, "USER"},
		{"email empty domain", "100014", "N", "a@", hash, "USER"},
		{"email trailing newline", "100015", "N", "a@x\n", hash, "USER"},
		{"email 255 chars", "100016", "N", strings.Repeat("a", 253) + "@b", hash, "USER"},
		{"user without email", "100017", "N", nil, hash, "USER"},
		{"admin with email", "100018", "N", "ad@x", hash, "ADMIN"},
		{"role unknown", "100019", "N", "e13@x", hash, "OWNER"},
		{"hash cost 12", "100020", "N", "e14@x", "$2a$12$" + hash[7:], "USER"},
		{"hash plain text", "100021", "N", "e15@x", strings.Repeat("p", 60), "USER"},
		{"duplicate emp_no", "089290", "N", "e16@x", hash, "USER"},
		{"duplicate email", "100022", "N", "a@b", hash, "USER"},
	}
	for _, c := range invalid {
		if err := insert(d, c.empNo, c.nm, c.email, c.pw, c.role); err == nil {
			t.Errorf("%s: want rejected, got accepted", c.name)
		}
	}
}

func TestAppUserEmailUniquenessIsByteExact(t *testing.T) {
	d := dbtest.New(t)
	if err := insert(d, "200001", "N", "a@x", hash, "USER"); err != nil {
		t.Fatal(err)
	}
	// Upper case is rejected by the CHECK, so the lower-cased form is the only one that can collide.
	if err := insert(d, "200002", "N", "a@x", hash, "USER"); err == nil {
		t.Fatal("duplicate lower-cased email accepted")
	}
}

func TestAppUserDefaults(t *testing.T) {
	d := dbtest.New(t)
	if err := insert(d, "300001", "N", "d@x", hash, "USER"); err != nil {
		t.Fatal(err)
	}
	var id string
	var enabled bool
	var created, updated sql.NullTime
	if err := d.QueryRow(`SELECT id, enabled, created_at, updated_at FROM app_user WHERE emp_no='300001'`).
		Scan(&id, &enabled, &created, &updated); err != nil {
		t.Fatal(err)
	}
	if len(id) != 36 || !enabled || !created.Valid || !updated.Valid {
		t.Fatalf("defaults wrong: id=%q enabled=%v created=%v updated=%v", id, enabled, created, updated)
	}
}
