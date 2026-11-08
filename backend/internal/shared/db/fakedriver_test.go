package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strconv"
	"sync"
	"sync/atomic"
)

var fakeSeq atomic.Int64

// fakeState scripts a driver and records what database/sql asked of it.
type script struct{ beginErr, commitErr, rollbackErr error }

type fakeState struct {
	script
	mu        sync.Mutex
	begun     int
	commits   int
	rollbacks int
	isolation driver.IsolationLevel
	txCtx     context.Context
}

func (s *fakeState) counts() (begun, commits, rollbacks int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.begun, s.commits, s.rollbacks
}

type fakeConn struct{ s *fakeState }

func (c fakeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c fakeConn) Close() error                        { return nil }
func (c fakeConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (c fakeConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.begun++
	c.s.isolation = driver.IsolationLevel(o.Isolation)
	c.s.txCtx = ctx
	if c.s.beginErr != nil {
		return nil, c.s.beginErr
	}
	return fakeTx(c), nil
}

type fakeTx struct{ s *fakeState }

func (t fakeTx) Commit() error {
	t.s.mu.Lock()
	defer t.s.mu.Unlock()
	t.s.commits++
	return t.s.commitErr
}

func (t fakeTx) Rollback() error {
	t.s.mu.Lock()
	defer t.s.mu.Unlock()
	t.s.rollbacks++
	return t.s.rollbackErr
}

type fakeDriver struct{ s *fakeState }

func (d fakeDriver) Open(string) (driver.Conn, error) { return fakeConn(d), nil }

func newFakeDB(s *fakeState) *sql.DB {
	name := "fake-" + strconv.FormatInt(fakeSeq.Add(1), 10)
	sql.Register(name, fakeDriver{s})
	d, err := sql.Open(name, "")
	if err != nil {
		panic(err)
	}
	return d
}
