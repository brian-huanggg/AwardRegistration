package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func srvErr(n uint16, state string) *mysql.MySQLError {
	e := &mysql.MySQLError{Number: n, Message: "x"}
	copy(e.SQLState[:], state)
	return e
}

func noop(context.Context, *sql.Tx) error { return nil }

func TestInTxOutcomes(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		state     script
		fn        func(context.Context, *sql.Tx) error
		want      Outcome
		wantErr   bool
		commits   int
		rollbacks int
	}{
		{"commit ok", script{}, noop, Committed, false, 1, 0},
		{"begin fails", script{beginErr: boom}, noop, RolledBack, true, 0, 0},
		{"fn error", script{}, func(context.Context, *sql.Tx) error { return boom }, RolledBack, true, 0, 1},
		{"fn error and rollback error", script{rollbackErr: boom},
			func(context.Context, *sql.Tx) error { return boom }, RolledBack, true, 0, 1},
		{"commit class 23 (integrity)", script{commitErr: srvErr(1062, "23000")}, noop, RolledBack, true, 1, 0},
		{"commit class 40 (deadlock)", script{commitErr: srvErr(1213, "40001")}, noop, RolledBack, true, 1, 0},
		{"commit other server error", script{commitErr: srvErr(1105, "HY000")}, noop, Unknown, true, 1, 0},
		{"commit bad conn", script{commitErr: driver.ErrBadConn}, noop, Unknown, true, 1, 0},
		{"commit invalid conn", script{commitErr: mysql.ErrInvalidConn}, noop, Unknown, true, 1, 0},
		{"commit generic error", script{commitErr: boom}, noop, Unknown, true, 1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeState{script: tc.state}
			got, err := InTx(context.Background(), newFakeDB(s), tc.fn)
			if got != tc.want {
				t.Errorf("outcome = %v, want %v", got, tc.want)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if _, c, r := s.counts(); c != tc.commits || r != tc.rollbacks {
				t.Errorf("commits=%d rollbacks=%d, want %d/%d", c, r, tc.commits, tc.rollbacks)
			}
		})
	}
}

func TestInTxUsesReadCommitted(t *testing.T) {
	s := &fakeState{}
	if _, err := InTx(context.Background(), newFakeDB(s), noop); err != nil {
		t.Fatal(err)
	}
	if s.isolation != driver.IsolationLevel(sql.LevelReadCommitted) {
		t.Errorf("isolation = %v, want READ COMMITTED", s.isolation)
	}
}

// A lock wait (1205) rolls back the whole transaction explicitly: MariaDB only
// rolls back the statement, so without Rollback earlier writes would commit.
func TestInTxLockWaitRollsBackWholeTx(t *testing.T) {
	s := &fakeState{}
	out, err := InTx(context.Background(), newFakeDB(s), func(context.Context, *sql.Tx) error {
		return srvErr(1205, "HY000")
	})
	if out != RolledBack {
		t.Errorf("outcome = %v", out)
	}
	if _, c, r := s.counts(); c != 0 || r != 1 {
		t.Errorf("commits=%d rollbacks=%d, want 0/1", c, r)
	}
	assertUnavailable(t, err)
}

func TestInTxContentionMapsTo503(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    script
		fn   func(context.Context, *sql.Tx) error
		out  Outcome
	}{
		{"1213 in fn", script{}, func(context.Context, *sql.Tx) error { return srvErr(1213, "40001") }, RolledBack},
		{"1205 at begin", script{beginErr: srvErr(1205, "HY000")}, noop, RolledBack},
		{"1213 at commit", script{commitErr: srvErr(1213, "40001")}, noop, RolledBack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeState{script: tc.s}
			out, err := InTx(context.Background(), newFakeDB(s), tc.fn)
			if out != tc.out {
				t.Errorf("outcome = %v, want %v", out, tc.out)
			}
			assertUnavailable(t, err)
		})
	}
}

func assertUnavailable(t *testing.T, err error) {
	t.Helper()
	var we *web.Error
	if !errors.As(err, &we) {
		t.Fatalf("err = %v, want *web.Error", err)
	}
	if we.Status != http.StatusServiceUnavailable || we.Code != web.CodeServiceUnavailable {
		t.Errorf("got %d %s, want 503 SERVICE_UNAVAILABLE", we.Status, we.Code)
	}
	if !IsContention(err) {
		t.Error("cause must stay reachable through errors.As")
	}
}

func TestInTxOtherServerErrorIsNotRemapped(t *testing.T) {
	want := srvErr(1062, "23000")
	_, err := InTx(context.Background(), newFakeDB(&fakeState{}), func(context.Context, *sql.Tx) error { return want })
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want the original error", err)
	}
	var we *web.Error
	if errors.As(err, &we) {
		t.Error("duplicate key must not be mapped by InTx")
	}
}

type ctxKey struct{}

// G8: cancelling the caller's context must not roll the transaction back.
func TestInTxSurvivesCallerCancel(t *testing.T) {
	s := &fakeState{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "kept"))
	out, err := InTx(ctx, newFakeDB(s), func(txCtx context.Context, _ *sql.Tx) error {
		cancel() // client disconnects while the transaction runs
		if txCtx.Err() != nil {
			t.Errorf("tx context cancelled with the caller: %v", txCtx.Err())
		}
		if txCtx.Value(ctxKey{}) != "kept" {
			t.Error("context values must be preserved")
		}
		return nil
	})
	if out != Committed || err != nil {
		t.Errorf("got %v, %v; want Committed, nil", out, err)
	}
}

func TestInTxCancelledBeforeStartStillRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := InTx(ctx, newFakeDB(&fakeState{}), noop); out != Committed || err != nil {
		t.Errorf("got %v, %v", out, err)
	}
}

func TestInTxHasOwnTimeout(t *testing.T) {
	s := &fakeState{}
	before := time.Now()
	_, _ = InTx(context.Background(), newFakeDB(s), func(ctx context.Context, _ *sql.Tx) error {
		dl, ok := ctx.Deadline()
		if !ok {
			t.Fatal("tx context has no deadline")
		}
		if d := dl.Sub(before); d <= 0 || d > TxTimeout+time.Second {
			t.Errorf("deadline in %v, want about %v", d, TxTimeout)
		}
		return nil
	})
}

func TestInTxTimeoutExpires(t *testing.T) {
	old := txTimeout
	txTimeout = 20 * time.Millisecond
	t.Cleanup(func() { txTimeout = old })

	s := &fakeState{}
	out, err := InTx(context.Background(), newFakeDB(s), func(ctx context.Context, _ *sql.Tx) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if out != RolledBack || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, %v; want RolledBack, DeadlineExceeded", out, err)
	}
}

func TestInTxPanicRollsBackAndRepanics(t *testing.T) {
	s := &fakeState{}
	defer func() {
		if r := recover(); r != "oops" {
			t.Errorf("recover = %v, want oops", r)
		}
		if _, c, rb := s.counts(); c != 0 || rb != 1 {
			t.Errorf("commits=%d rollbacks=%d, want 0/1", c, rb)
		}
	}()
	_, _ = InTx(context.Background(), newFakeDB(s), func(context.Context, *sql.Tx) error { panic("oops") })
}

func TestOutcomeString(t *testing.T) {
	for o, want := range map[Outcome]string{Committed: "Committed", RolledBack: "RolledBack", Unknown: "Unknown", 9: "Outcome(9)"} {
		if o.String() != want {
			t.Errorf("%d = %q, want %q", int(o), o.String(), want)
		}
	}
}

func TestSQLStateClassNonMySQL(t *testing.T) {
	if sqlStateClass(errors.New("x")) != "" || IsContention(errors.New("x")) {
		t.Error("non-MySQL error must have no class and no contention")
	}
}

func TestDuplicateKey(t *testing.T) {
	dup := func(msg string) error { return fmt.Errorf("wrap: %w", &mysql.MySQLError{Number: 1062, Message: msg}) }
	cases := []struct {
		name string
		err  error
		key  string
		ok   bool
	}{
		{"plain", dup("Duplicate entry 'a@b' for key 'uq_app_user_email'"), "uq_app_user_email", true},
		{"table qualified", dup("Duplicate entry '1' for key 'app_user.uq_app_user_emp_no'"), "uq_app_user_emp_no", true},
		{"value contains the marker", dup("Duplicate entry 'for key ''x' for key 'uq_a'"), "uq_a", true},
		{"no key in message", dup("boom"), "", true},
		{"unterminated", dup("Duplicate entry 'x' for key 'uq_a"), "uq_a", true},
		{"other number", &mysql.MySQLError{Number: 1205}, "", false},
		{"not a mysql error", errors.New("x"), "", false},
	}
	for _, tc := range cases {
		key, ok := DuplicateKey(tc.err)
		if key != tc.key || ok != tc.ok {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", tc.name, key, ok, tc.key, tc.ok)
		}
	}
}
