//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

// purgeFixture seeds every purged table with rows that must stay and rows that
// must go. Each "dead" group is larger than the batch size so the drain loop
// runs several DELETEs.
type purgeFixture struct {
	liveSessions []string
	deadSessions int
}

func seedPurge(t *testing.T, db *sql.DB, s *auth.Sessions) purgeFixture {
	t.Helper()
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	var f purgeFixture
	u := addUser(t, db, "200001", "USER", true)
	for i := range 3 {
		id, err := s.Create(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 { // inside both timeouts, but close to both
			age(t, db, id, 7*time.Hour+50*time.Minute, 29*time.Minute)
		}
		f.liveSessions = append(f.liveSessions, id)
	}
	for i := range 7 {
		id, err := s.Create(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		if i%2 == 0 {
			age(t, db, id, time.Hour, 31*time.Minute) // idle timeout
		} else {
			age(t, db, id, 8*time.Hour+time.Minute, 0) // absolute timeout
		}
		f.deadSessions++
	}

	lock := func(key string, lastFailAgo time.Duration, lockedUntil string) {
		exec(`INSERT INTO login_lock (lock_key, fail_count, last_fail_at, locked_until)
		      VALUES (?, 5, NOW(6) - INTERVAL ? SECOND, `+lockedUntil+`)`, key, int(lastFailAgo.Seconds()))
	}
	lock("keep-recent", time.Minute, "NULL")                              // counting window still open
	lock("keep-locked", 20*time.Minute, "NOW(6) + INTERVAL 5 MINUTE")     // lock active
	lock("keep-just-unlocked", time.Minute, "NOW(6) - INTERVAL 1 SECOND") // window still open
	for i := range 5 {
		lock(fmt.Sprintf("dead-stale-%d", i), 16*time.Minute, "NULL")
		lock(fmt.Sprintf("dead-unlocked-%d", i), 31*time.Minute, "NOW(6) - INTERVAL 1 MINUTE")
	}

	for range 4 {
		exec(`INSERT INTO login_attempt (ip, attempted_at) VALUES ('192.0.2.1', NOW(6) - INTERVAL 14 MINUTE)`)
	}
	for range 9 {
		exec(`INSERT INTO login_attempt (ip, attempted_at) VALUES ('192.0.2.2', NOW(6) - INTERVAL 901 SECOND)`)
	}

	exec(`INSERT INTO login_throttle_audit VALUES ('ip', 'keep', NOW(6) + INTERVAL 1 MINUTE)`)
	for i := range 5 {
		exec(`INSERT INTO login_throttle_audit VALUES ('key', ?, NOW(6) - INTERVAL 1 SECOND)`, fmt.Sprintf("dead-%d", i))
	}
	return f
}

func count(t *testing.T, db *sql.DB, q string) (n int) {
	t.Helper()
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func assertPurged(t *testing.T, db *sql.DB, s *auth.Sessions, f purgeFixture) {
	t.Helper()
	if n := count(t, db, `SELECT COUNT(*) FROM session`); n != len(f.liveSessions) {
		t.Errorf("sessions = %d, want %d", n, len(f.liveSessions))
	}
	for _, id := range f.liveSessions {
		if _, ok, err := s.Validate(context.Background(), id); err != nil || !ok {
			t.Errorf("live session lost: %v", err)
		}
	}
	rows, err := db.Query(`SELECT lock_key FROM login_lock ORDER BY lock_key`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	if got := strings.Join(keys, ","); got != "keep-just-unlocked,keep-locked,keep-recent" {
		t.Errorf("login_lock keys = %s", got)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM login_attempt WHERE ip = '192.0.2.1'`); n != 4 {
		t.Errorf("attempts inside window = %d, want 4", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM login_attempt WHERE ip = '192.0.2.2'`); n != 0 {
		t.Errorf("attempts outside window = %d, want 0", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM login_throttle_audit WHERE subject = 'keep'`); n != 1 {
		t.Errorf("active throttle claim lost")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM login_throttle_audit`); n != 1 {
		t.Errorf("throttle claims = %d, want 1", n)
	}
}

func TestPurgeDeletesDeadRowsOnly(t *testing.T) {
	db := dbtest.New(t)
	s := auth.New(db)
	f := seedPurge(t, db, s)
	var buf bytes.Buffer
	p := auth.Purger{DB: db, Log: slog.New(slog.NewTextHandler(&buf, nil)), Batch: 2}
	if err := p.Purge(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPurged(t, db, s, f)
	for _, want := range []string{
		"table=session deleted=7", "table=login_lock deleted=10",
		"table=login_attempt deleted=9", "table=login_throttle_audit deleted=5",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log lacks %q: %s", want, buf.String())
		}
	}

	// Idempotent: a second pass finds nothing and logs nothing.
	buf.Reset()
	if err := p.Purge(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPurged(t, db, s, f)
	if buf.Len() != 0 {
		t.Errorf("second pass logged: %s", buf.String())
	}
}

// Every replica runs the purge with no leader. Several runners on the same
// rows must neither fail nor delete a row that is still live.
func TestPurgeConcurrentRunnersDoNotConflict(t *testing.T) {
	db := dbtest.New(t)
	s := auth.New(db)
	f := seedPurge(t, db, s)
	const runners = 4
	var wg sync.WaitGroup
	errs := make(chan error, runners)
	start := make(chan struct{})
	for range runners {
		wg.Go(func() {
			<-start
			p := auth.Purger{DB: db, Log: slog.New(slog.DiscardHandler), Batch: 1}
			errs <- p.Purge(context.Background())
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("runner failed: %v", err)
		}
	}
	assertPurged(t, db, s, f)
}

func TestPurgeStopsOnCancel(t *testing.T) {
	db := dbtest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (auth.Purger{DB: db, Log: slog.New(slog.DiscardHandler)}).Purge(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// A database error on one table is returned, but the other tables still get
// their turn and each failure is logged.
func TestPurgeReportsDatabaseErrors(t *testing.T) {
	db, err := sql.Open("mysql", dbtest.DSN("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	var buf bytes.Buffer
	err = (auth.Purger{DB: db, Log: slog.New(slog.NewTextHandler(&buf, nil))}).Purge(context.Background())
	if err == nil || !strings.Contains(err.Error(), "purge sessions") {
		t.Fatalf("err = %v", err)
	}
	if n := strings.Count(buf.String(), "purge failed"); n != 4 {
		t.Errorf("logged %d failures, want 4: %s", n, buf.String())
	}
}
