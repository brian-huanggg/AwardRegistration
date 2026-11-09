package dbtest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// CommitFault is what the next COMMIT on a FaultyDB does (Backend.md §11,
// Sequence.md §7: lost-commit tests).
type CommitFault struct {
	// Apply commits the transaction on the server before Err is returned; when
	// false the transaction is rolled back instead and nothing is applied.
	Apply bool
	// Err is what Commit returns to database/sql.
	Err error
}

var (
	// LostAckCommitted: the server committed, the acknowledgement was lost.
	LostAckCommitted = CommitFault{Apply: true, Err: mysql.ErrInvalidConn}
	// LostAckNotCommitted: the connection died before the COMMIT was applied.
	LostAckNotCommitted = CommitFault{Apply: false, Err: mysql.ErrInvalidConn}
	// DeadlockAtCommit: the server rejected the COMMIT with SQLState 40001.
	DeadlockAtCommit = CommitFault{Apply: false, Err: &mysql.MySQLError{
		Number: 1213, SQLState: [5]byte{'4', '0', '0', '0', '1'}, Message: "Deadlock found (injected)"}}
)

// Faults arms commit faults on a pool opened by OpenFaulty. A fault applies to
// the next COMMIT on any connection of that pool, then disarms itself.
type Faults struct {
	mu    sync.Mutex
	next  *CommitFault
	fired int
}

// FailNextCommit arms f for the next COMMIT.
func (f *Faults) FailNextCommit(cf CommitFault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next = &cf
}

// Fired reports how many armed faults have been triggered.
func (f *Faults) Fired() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fired
}

func (f *Faults) take() *CommitFault {
	f.mu.Lock()
	defer f.mu.Unlock()
	cf := f.next
	if cf != nil {
		f.next = nil
		f.fired++
	}
	return cf
}

// OpenFaulty opens a pool on dsn whose driver.Connector wraps the MySQL one, so
// a COMMIT can be made to "commit then fail" or "fail without committing". The
// pool is closed when the test ends.
func OpenFaulty(t testing.TB, dsn string) (*sql.DB, *Faults) {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := mysql.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	f := &Faults{}
	db := sql.OpenDB(faultConnector{inner: inner, f: f})
	t.Cleanup(func() { _ = db.Close() })
	return db, f
}

type faultConnector struct {
	inner driver.Connector
	f     *Faults
}

func (c faultConnector) Driver() driver.Driver { return c.inner.Driver() }

func (c faultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	fc, ok := conn.(fullConn)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("dbtest: %T lacks a driver interface the wrapper forwards", conn)
	}
	return faultConn{fullConn: fc, f: c.f}, nil
}

// fullConn is every optional interface the MySQL connection implements;
// embedding it forwards them all, so database/sql takes the same code paths as
// with the plain driver.
type fullConn interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.QueryerContext
	driver.Pinger
	driver.SessionResetter
	driver.Validator
	driver.NamedValueChecker
}

type faultConn struct {
	fullConn
	f *Faults
}

func (c faultConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c faultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := c.fullConn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return faultTx{inner: tx, f: c.f}, nil
}

type faultTx struct {
	inner driver.Tx
	f     *Faults
}

func (t faultTx) Rollback() error { return t.inner.Rollback() }

func (t faultTx) Commit() error {
	cf := t.f.take()
	if cf == nil {
		return t.inner.Commit()
	}
	if cf.Apply {
		if err := t.inner.Commit(); err != nil {
			return fmt.Errorf("dbtest: real commit failed before the injected fault: %w", err)
		}
		return cf.Err
	}
	// Roll back so the locks are released; the server never applies it.
	if err := t.inner.Rollback(); err != nil {
		return errors.Join(cf.Err, err)
	}
	return cf.Err
}
