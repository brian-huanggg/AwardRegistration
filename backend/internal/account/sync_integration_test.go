//go:build integration

package account_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
)

type fakeDir struct {
	mu   sync.Mutex
	snap account.AdminSnapshot
	err  error
}

func (d *fakeDir) set(snap account.AdminSnapshot, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.snap, d.err = snap, err
}

func (d *fakeDir) List(context.Context) (account.AdminSnapshot, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.snap, d.err
}

func hash10(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func newSync(e env, dir account.AdminDirectory) *account.AdminSync {
	return &account.AdminSync{DB: e.db, Dir: dir, Audit: audit.New(e.db, nil), Log: slog.New(slog.DiscardHandler)}
}

type adminRow struct {
	Name, Hash string
	Role       string
	Enabled    bool
	Email      *string
}

func (e env) admin(t *testing.T, empNo string) (adminRow, bool) {
	t.Helper()
	var r adminRow
	err := e.db.QueryRow(`SELECT name, password_hash, role, enabled, email FROM app_user WHERE emp_no = ?`, empNo).
		Scan(&r.Name, &r.Hash, &r.Role, &r.Enabled, &r.Email)
	if err != nil {
		return adminRow{}, false
	}
	return r, true
}

func (e env) syncRows(t *testing.T) []auditRow {
	t.Helper()
	var out []auditRow
	for _, r := range e.auditRows(t) {
		if r.Action == "ADMIN_SYNC" {
			out = append(out, r)
		}
	}
	return out
}

func TestAdminSyncAddsAdminAndBlocksRegistration(t *testing.T) {
	e := newEnv(t)
	h := hash10(t, "pw")
	dir := &fakeDir{snap: account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada", Hash: h}}}}
	s := newSync(e, dir)

	if s.Ready() || s.ReadyCheck(context.Background()) == nil {
		t.Fatal("ready before the first read")
	}
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !s.Ready() || s.ReadyCheck(context.Background()) != nil {
		t.Fatal("not ready after the first read")
	}
	a, ok := e.admin(t, "900001")
	if !ok || a.Role != "ADMIN" || !a.Enabled || a.Email != nil || a.Name != "Ada" || a.Hash != h {
		t.Fatalf("admin row = %+v ok=%v", a, ok)
	}

	// An unchanged directory writes nothing.
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := e.syncRows(t)
	if len(rows) != 1 || rows[0].Outcome != "SUCCESS" || rows[0].ActorRole != "SYSTEM" || rows[0].TargetType.String != "USER" ||
		!strings.Contains(rows[0].After.String, `"added"`) || strings.Contains(rows[0].After.String, h) {
		t.Fatalf("audit rows = %+v", rows)
	}

	rec := e.register(reg("900001", "Mallory", "m@example.com", goodPW))
	if rec.Code != 409 || problemCode(t, rec) != "EMP_NO_TAKEN" {
		t.Fatalf("register as Admin: %d %s", rec.Code, rec.Body)
	}
}

func TestAdminSyncTwoReplicasWriteOnce(t *testing.T) {
	e := newEnv(t)
	h1, h2 := hash10(t, "one"), hash10(t, "two")
	dir := &fakeDir{}
	a, b := newSync(e, dir), newSync(e, dir)

	race := func() {
		var wg sync.WaitGroup
		for _, s := range []*account.AdminSync{a, b, a, b} {
			wg.Add(1)
			go func() { defer wg.Done(); _ = s.Run(context.Background()) }()
		}
		wg.Wait()
	}
	count := func(want int, step string) {
		t.Helper()
		if got := len(e.syncRows(t)); got != want {
			t.Fatalf("%s: %d ADMIN_SYNC rows, want %d: %+v", step, got, want, e.syncRows(t))
		}
	}

	dir.set(account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada", Hash: h1}}}, nil)
	race()
	count(1, "add")

	dir.set(account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada L", Hash: h2}}}, nil)
	race()
	count(2, "update")
	if r, _ := e.admin(t, "900001"); r.Name != "Ada L" || r.Hash != h2 {
		t.Fatalf("not updated: %+v", r)
	}

	dir.set(account.AdminSnapshot{}, nil)
	race()
	count(3, "disable")
	if r, _ := e.admin(t, "900001"); r.Enabled {
		t.Fatal("removed Admin still enabled")
	}

	dir.set(account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada L", Hash: h2}}}, nil)
	race()
	count(4, "re-enable")
	if r, _ := e.admin(t, "900001"); !r.Enabled {
		t.Fatal("rejoined Admin not re-enabled")
	}
	if !strings.Contains(e.syncRows(t)[3].After.String, "re-enabled") {
		t.Fatalf("row = %+v", e.syncRows(t)[3])
	}
}

func TestAdminSyncSkipsUserConflict(t *testing.T) {
	e := newEnv(t)
	if rec := e.register(reg("900002", "Bob", "bob@example.com", goodPW)); rec.Code != 201 {
		t.Fatal(rec.Body)
	}
	h := hash10(t, "pw")
	dir := &fakeDir{snap: account.AdminSnapshot{Entries: []account.AdminEntry{
		{EmpNo: "900002", Name: "Hijack", Hash: h},
		{EmpNo: "900001", Name: "Ada", Hash: h},
	}}}
	s := newSync(e, dir)
	for range 3 {
		if err := s.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if r, _ := e.admin(t, "900002"); r.Role != "USER" || r.Name != "Bob" || r.Hash == h {
		t.Fatalf("User converted or changed: %+v", r)
	}
	if r, ok := e.admin(t, "900001"); !ok || r.Role != "ADMIN" {
		t.Fatal("other entries must still sync")
	}
	if !s.Ready() || s.Skipped() != 3 {
		t.Fatalf("ready=%v skipped=%d", s.Ready(), s.Skipped())
	}
	var failed int
	for _, r := range e.syncRows(t) {
		if r.Outcome == "FAILED" && r.ErrorCode == account.SkipUserConflict {
			failed++
		}
	}
	if failed != 1 {
		t.Fatalf("FAILED rows = %d, want 1 (a standing conflict is audited once): %+v", failed, e.syncRows(t))
	}
}

func TestAdminSyncKeepsAdminsOnFailure(t *testing.T) {
	e := newEnv(t)
	h := hash10(t, "pw")
	dir := &fakeDir{err: errors.New("vault down")}
	s := newSync(e, dir)

	if err := s.Run(context.Background()); err == nil || s.Ready() || s.Failures() != 1 {
		t.Fatalf("first read failure: err=%v ready=%v failures=%d", err, s.Ready(), s.Failures())
	}

	dir.set(account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada", Hash: h}}}, nil)
	if err := s.Run(context.Background()); err != nil || !s.Ready() {
		t.Fatalf("err=%v ready=%v", err, s.Ready())
	}

	dir.set(account.AdminSnapshot{}, errors.New("vault down"))
	if err := s.Run(context.Background()); err == nil || s.Failures() != 2 {
		t.Fatalf("err=%v failures=%d", err, s.Failures())
	}
	if r, _ := e.admin(t, "900001"); !r.Enabled {
		t.Fatal("Admin disabled by a failed sync")
	}
	if !s.Ready() {
		t.Fatal("a later failure must not make the service unready")
	}
}

func TestAdminSyncRejectedEntryIsNotRemoval(t *testing.T) {
	e := newEnv(t)
	h := hash10(t, "pw")
	dir := &fakeDir{snap: account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada", Hash: h}}}}
	s := newSync(e, dir)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	dir.set(account.AdminSnapshot{Rejected: []account.RejectedEntry{{EmpNo: "900001", Reason: account.RejectCost}}}, nil)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r, _ := e.admin(t, "900001"); !r.Enabled || r.Hash != h {
		t.Fatalf("rejected entry changed the Admin: %+v", r)
	}
	rows := e.syncRows(t)
	if last := rows[len(rows)-1]; last.Outcome != "FAILED" || last.ErrorCode != account.RejectCost {
		t.Fatalf("last row = %+v", last)
	}
}

func TestAdminSyncSkipsEntryTheTableRefuses(t *testing.T) {
	e := newEnv(t)
	h := hash10(t, "pw")
	dir := &fakeDir{snap: account.AdminSnapshot{Entries: []account.AdminEntry{
		{EmpNo: "900001", Name: "a/b", Hash: h}, // '/' is refused by ck_app_user_name_chars
		{EmpNo: "900002", Name: "Ok", Hash: h},
	}}}
	s := newSync(e, dir)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(fmt.Sprint(err))
	}
	if _, ok := e.admin(t, "900001"); ok {
		t.Fatal("bad entry stored")
	}
	if _, ok := e.admin(t, "900002"); !ok || !s.Ready() {
		t.Fatal("good entry must sync and the service must be ready")
	}
}

// flakyRevoker wraps the real session store and fails the first n calls.
type flakyRevoker struct {
	real  account.SessionRevoker
	mu    sync.Mutex
	fails int
	calls int
}

func (r *flakyRevoker) RevokeAll(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	r.calls++
	fail := r.fails > 0
	if fail {
		r.fails--
	}
	r.mu.Unlock()
	if fail {
		return errors.New("revoke down")
	}
	return r.real.RevokeAll(ctx, id)
}

func (e env) adminID(t *testing.T, empNo string) uuid.UUID {
	t.Helper()
	var id string
	if err := e.db.QueryRow(`SELECT id FROM app_user WHERE emp_no = ?`, empNo).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return uuid.MustParse(id)
}

func TestAdminSyncRevokesSessions(t *testing.T) {
	e := newEnv(t)
	sessions := auth.New(e.db)
	h1, h2 := hash10(t, "one"), hash10(t, "two")
	dir := &fakeDir{snap: account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada", Hash: h1}}}}
	rev := &flakyRevoker{real: sessions}
	s := newSync(e, dir)
	s.Revoker = rev
	ctx := context.Background()
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	id := e.adminID(t, "900001")
	live := func(sid string) bool {
		t.Helper()
		_, ok, err := sessions.Validate(ctx, sid)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	newSession := func() string {
		t.Helper()
		sid, err := sessions.Create(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return sid
	}

	// Unchanged directory and a name-only change keep the session.
	sid := newSession()
	dir.set(account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada L", Hash: h1}}}, nil)
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if !live(sid) || rev.calls != 0 {
		t.Fatalf("name change revoked sessions (calls=%d)", rev.calls)
	}

	// Hash change revokes.
	dir.set(account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada L", Hash: h2}}}, nil)
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if live(sid) {
		t.Fatal("session survived a hash change")
	}
	if rows := e.syncRows(t); !strings.Contains(rows[len(rows)-1].After.String, `"sessionsRevoked":true`) {
		t.Fatalf("audit row = %+v", rows[len(rows)-1])
	}

	// Removal: the next request is refused.
	sid = newSession()
	dir.set(account.AdminSnapshot{}, nil)
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if live(sid) {
		t.Fatal("removed Admin's session still valid")
	}
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM session WHERE user_id = ?`, id.String()).Scan(&n); err != nil || n != 0 {
		t.Fatalf("session rows after disable = %d, err %v", n, err)
	}
}

func TestAdminSyncRetriesFailedRevoke(t *testing.T) {
	e := newEnv(t)
	sessions := auth.New(e.db)
	h1, h2 := hash10(t, "one"), hash10(t, "two")
	dir := &fakeDir{snap: account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada", Hash: h1}}}}
	rev := &flakyRevoker{real: sessions, fails: 1}
	s := newSync(e, dir)
	s.Revoker = rev
	ctx := context.Background()
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	sid, err := sessions.Create(ctx, e.adminID(t, "900001"))
	if err != nil {
		t.Fatal(err)
	}

	dir.set(account.AdminSnapshot{Entries: []account.AdminEntry{{EmpNo: "900001", Name: "Ada", Hash: h2}}}, nil)
	if err := s.Run(ctx); err != nil {
		t.Fatal(err) // a failed revoke does not fail the run
	}
	if _, ok, _ := sessions.Validate(ctx, sid); !ok {
		t.Fatal("test setup: first revoke should have failed")
	}
	// The hash is already stored, so only the retry can revoke now.
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := sessions.Validate(ctx, sid); ok {
		t.Fatal("failed revoke was not retried")
	}
}
