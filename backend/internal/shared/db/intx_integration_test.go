//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/toxiproxy"
	"github.com/testcontainers/testcontainers-go/network"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

const toxiproxyImage = "ghcr.io/shopify/toxiproxy:2.12.0"

// insertLock writes one login_lock row (PK lock_key): a table with a primary
// key, so a second writer blocks while the first transaction is still open.
func insertLock(ctx context.Context, tx *sql.Tx, key string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO login_lock (lock_key, fail_count, last_fail_at) VALUES (?, 1, NOW(6))`, key)
	return err
}

func lockRows(t *testing.T, d *sql.DB, key string) int {
	t.Helper()
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM login_lock WHERE lock_key = ?`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Backend.md §4.4 / §11: the classification against a real server, through the
// connector that commits-then-fails or fails-without-committing.
func TestInTxLostCommitAgainstMariaDB(t *testing.T) {
	dsn := dbtest.Start(t)
	direct := dbtest.Open(t, dsn)
	faulty, faults := dbtest.OpenFaulty(t, dsn)
	ctx := context.Background()

	for _, tc := range []struct {
		name    string
		fault   *dbtest.CommitFault
		want    db.Outcome
		applied bool
	}{
		{"no fault", nil, db.Committed, true},
		{"commit applied, ack lost", &dbtest.LostAckCommitted, db.Unknown, true},
		{"connection lost before commit", &dbtest.LostAckNotCommitted, db.Unknown, false},
		{"deadlock at commit (SQLState 40)", &dbtest.DeadlockAtCommit, db.RolledBack, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := "lost-" + strings.ReplaceAll(tc.name, " ", "-")
			if len(key) > 32 {
				key = key[:32]
			}
			if tc.fault != nil {
				faults.FailNextCommit(*tc.fault)
			}
			out, err := db.InTx(ctx, faulty, func(ctx context.Context, tx *sql.Tx) error {
				return insertLock(ctx, tx, key)
			})
			if out != tc.want {
				t.Errorf("outcome = %v, want %v (err %v)", out, tc.want, err)
			}
			if (err != nil) != (tc.fault != nil) {
				t.Errorf("err = %v", err)
			}
			if got := lockRows(t, direct, key) == 1; got != tc.applied {
				t.Errorf("row applied = %v, want %v", got, tc.applied)
			}
		})
	}

	t.Run("fn error rolls back on the real server", func(t *testing.T) {
		out, err := db.InTx(ctx, faulty, func(ctx context.Context, tx *sql.Tx) error {
			if err := insertLock(ctx, tx, "lost-fn-error"); err != nil {
				return err
			}
			return errors.New("audit insert failed")
		})
		if out != db.RolledBack || err == nil || lockRows(t, direct, "lost-fn-error") != 0 {
			t.Errorf("got %v, %v", out, err)
		}
	})
}

// One real-network sanity check (Sequence.md §7): Toxiproxy cuts the
// connection between the last statement and COMMIT. The driver sees a dead
// connection, InTx must say Unknown (never RolledBack: it cannot know), and the
// server must have rolled the transaction back.
func TestInTxConnectionCutBeforeCommit(t *testing.T) {
	ctx := context.Background()
	nw, err := network.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = nw.Remove(context.WithoutCancel(ctx)) })

	direct := dbtest.Open(t, dbtest.Start(t, network.WithNetwork([]string{"mariadb"}, nw)))

	tp, err := toxiproxy.Run(ctx, toxiproxyImage,
		toxiproxy.WithProxy("mariadb", "mariadb:3306"),
		network.WithNetwork([]string{"toxiproxy"}, nw))
	testcontainers.CleanupContainer(t, tp)
	if err != nil {
		t.Fatalf("start toxiproxy: %v", err)
	}
	host, port, err := tp.ProxiedEndpoint(8666)
	if err != nil {
		t.Fatal(err)
	}
	api, err := tp.URI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	proxied := dbtest.Open(t, dbtest.DSN(net.JoinHostPort(host, port)))
	if err := proxied.PingContext(ctx); err != nil {
		t.Fatalf("ping through toxiproxy: %v", err)
	}

	const key = "toxiproxy-cut"
	out, err := db.InTx(ctx, proxied, func(ctx context.Context, tx *sql.Tx) error {
		if err := insertLock(ctx, tx, key); err != nil {
			return err
		}
		disableProxy(t, api) // the next bytes (COMMIT) go nowhere
		return nil
	})
	if out != db.Unknown || err == nil {
		t.Fatalf("InTx = %v, %v; want Unknown with an error", out, err)
	}

	// COMMIT never reached the server: once it notices the closed socket it
	// rolls back and releases the row lock, so this insert stops blocking.
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := direct.ExecContext(ctx,
			`SET STATEMENT innodb_lock_wait_timeout = 1 FOR
			 INSERT INTO login_lock (lock_key, fail_count, last_fail_at) VALUES (?, 1, NOW(6))`, key)
		if err == nil {
			break
		}
		if !db.IsContention(err) || time.Now().After(deadline) {
			t.Fatalf("server did not roll the cut transaction back: %v", err)
		}
	}
}

// disableProxy disables the proxy over the Toxiproxy HTTP API, which closes
// every connection through it.
func disableProxy(t *testing.T, api string) {
	t.Helper()
	body := `{"enabled":false}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		api+"/proxies/mariadb", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "toxiproxy-cli")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("toxiproxy update: HTTP %d", resp.StatusCode)
	}
}
