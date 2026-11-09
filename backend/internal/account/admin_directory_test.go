package account

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func hashAt(t *testing.T, cost int) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte("pw"), cost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func writeDir(t *testing.T, body string) FileAdminDirectory {
	t.Helper()
	p := filepath.Join(t.TempDir(), "admins.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return FileAdminDirectory{Path: p}
}

func TestFileAdminDirectoryEntryRules(t *testing.T) {
	h10, h4, h12 := hashAt(t, 10), hashAt(t, bcrypt.MinCost), hashAt(t, 12)
	body := `[
	 {"empNo":"900001","name":" Ada ","hash":"` + h10 + `"},
	 {"empNo":"900002","name":"Low","hash":"` + h4 + `"},
	 {"empNo":"900003","name":"High","hash":"` + h12 + `"},
	 {"empNo":"90004","name":"Short","hash":"` + h10 + `"},
	 {"empNo":"900005","name":"  ","hash":"` + h10 + `"},
	 {"empNo":"900006","name":"Bad","hash":"not-a-hash"},
	 {"empNo":"900001","name":"Dup","hash":"` + h10 + `"},
	 {"name":"NoEmp","hash":"` + h10 + `"},
	 {"empNo":"900007","name":"Ok","hash":"` + h10 + `"}
	]`
	snap, err := writeDir(t, body).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Entries) != 2 || snap.Entries[0] != (AdminEntry{"900001", "Ada", h10}) || snap.Entries[1].EmpNo != "900007" {
		t.Fatalf("entries = %+v", snap.Entries)
	}
	want := []RejectedEntry{
		{"900002", RejectCost}, {"900003", RejectCost}, {"90004", RejectEmpNo},
		{"900005", RejectName}, {"900006", RejectHash}, {"900001", RejectDuplicate},
		{"", RejectEmpNo},
	}
	if len(snap.Rejected) != len(want) {
		t.Fatalf("rejected = %+v", snap.Rejected)
	}
	for i, w := range want {
		if snap.Rejected[i] != w {
			t.Errorf("rejected[%d] = %+v, want %+v", i, snap.Rejected[i], w)
		}
	}
}

func TestFileAdminDirectoryWholeReadFailures(t *testing.T) {
	for name, d := range map[string]FileAdminDirectory{
		"missing":   {Path: filepath.Join(t.TempDir(), "nope.json")},
		"malformed": writeDir(t, `{"not":"a list"`),
		"wrongtype": writeDir(t, `{"empNo":"900001"}`),
	} {
		if _, err := d.List(context.Background()); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := writeDir(t, `[]`).List(ctx); err == nil {
		t.Error("canceled ctx: want error")
	}
}

func TestFileAdminDirectoryEmptyList(t *testing.T) {
	snap, err := writeDir(t, `[]`).List(context.Background())
	if err != nil || len(snap.Entries) != 0 || len(snap.Rejected) != 0 {
		t.Fatalf("snap=%+v err=%v", snap, err)
	}
}

var _ AdminDirectory = FileAdminDirectory{}
