//go:build integration

package storage_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
)

// fakeStore records Delete calls; deleteErr makes them fail (MinIO down).
type fakeStore struct {
	mu        sync.Mutex
	deleteErr error
	deleted   []string
	ctxErr    []error // ctx.Err() seen by each Delete
}

func (f *fakeStore) Put(context.Context, string, io.Reader, int64, string) error { return nil }
func (f *fakeStore) Get(context.Context, string) (io.ReadCloser, int64, error) {
	return nil, 0, storage.ErrObjectNotFound
}

func (f *fakeStore) Delete(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctxErr = append(f.ctxErr, ctx.Err())
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, key)
	return nil
}

type pendingRow struct {
	Reason   string
	DueInSec float64 // next_attempt_at - NOW(6), database time
	Attempts int
}

func pendingRows(t *testing.T, d *sql.DB, key string) []pendingRow {
	t.Helper()
	rows, err := d.Query(`SELECT reason, TIMESTAMPDIFF(MICROSECOND, NOW(6), next_attempt_at) / 1e6, attempts
	                        FROM pending_file_deletion WHERE object_key = ? ORDER BY id`, key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.Reason, &r.DueInSec, &r.Attempts); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func newLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func TestPendingDeletion(t *testing.T) {
	dsn := dbtest.Start(t)
	d := dbtest.Open(t, dsn)
	ctx := context.Background()

	recordInTx := func(t *testing.T, p *storage.PendingDeletion, key string, r storage.Reason) storage.PendingID {
		t.Helper()
		var id storage.PendingID
		out, err := db.InTx(ctx, d, func(ctx context.Context, tx *sql.Tx) error {
			var err error
			id, err = p.RecordInTx(ctx, tx, key, r)
			return err
		})
		if out != db.Committed || err != nil {
			t.Fatalf("InTx = %v, %v", out, err)
		}
		return id
	}

	t.Run("RecordInTx writes REPLACED / DELETED due now, only when the tx commits", func(t *testing.T) {
		p := storage.NewPendingDeletion(d, &fakeStore{}, nil)
		for _, r := range []storage.Reason{storage.ReasonReplaced, storage.ReasonDeleted} {
			key := "slides/in-tx/" + string(r) + ".pdf"
			if id := recordInTx(t, p, key, r); id <= 0 {
				t.Errorf("%s: id = %d", r, id)
			}
			got := pendingRows(t, d, key)
			if len(got) != 1 || got[0].Reason != string(r) || got[0].DueInSec > 0 || got[0].Attempts != 0 {
				t.Errorf("%s: rows = %+v, want one due row", r, got)
			}
		}

		key := "slides/in-tx/rolled-back.pdf"
		out, _ := db.InTx(ctx, d, func(ctx context.Context, tx *sql.Tx) error {
			if _, err := p.RecordInTx(ctx, tx, key, storage.ReasonReplaced); err != nil {
				return err
			}
			return errors.New("later step fails")
		})
		if out != db.RolledBack {
			t.Fatalf("outcome = %v", out)
		}
		if got := pendingRows(t, d, key); len(got) != 0 {
			t.Errorf("rolled-back tx left %+v", got)
		}
	})

	t.Run("RecordUploadAborted: known rollback due now, unknown after the grace period", func(t *testing.T) {
		p := storage.NewPendingDeletion(d, &fakeStore{}, nil)
		if err := p.RecordUploadAborted(ctx, "slides/aborted/rb.pdf", db.RolledBack); err != nil {
			t.Fatal(err)
		}
		if got := pendingRows(t, d, "slides/aborted/rb.pdf"); len(got) != 1 ||
			got[0].Reason != "UPLOAD_ABORTED" || got[0].DueInSec > 0 {
			t.Errorf("rows = %+v", got)
		}

		if err := p.RecordUploadAborted(ctx, "slides/aborted/unknown.pdf", db.Unknown); err != nil {
			t.Fatal(err)
		}
		got := pendingRows(t, d, "slides/aborted/unknown.pdf")
		grace := storage.UploadAbortedGrace.Seconds()
		if len(got) != 1 || got[0].Reason != "UPLOAD_ABORTED" || got[0].DueInSec > grace || got[0].DueInSec < grace-60 {
			t.Errorf("rows = %+v, want due in about %v s", got, grace)
		}
	})

	t.Run("RecordUploadAborted survives a cancelled request context", func(t *testing.T) {
		p := storage.NewPendingDeletion(d, &fakeStore{}, nil)
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		if err := p.RecordUploadAborted(cctx, "slides/aborted/cancelled.pdf", db.Unknown); err != nil {
			t.Fatal(err)
		}
		if got := pendingRows(t, d, "slides/aborted/cancelled.pdf"); len(got) != 1 {
			t.Errorf("rows = %+v", got)
		}
	})

	t.Run("a key may be recorded more than once", func(t *testing.T) {
		p := storage.NewPendingDeletion(d, &fakeStore{}, nil)
		key := "slides/twice/k.pdf"
		if err := p.RecordUploadAborted(ctx, key, db.Unknown); err != nil {
			t.Fatal(err)
		}
		recordInTx(t, p, key, storage.ReasonReplaced)
		if got := pendingRows(t, d, key); len(got) != 2 {
			t.Errorf("rows = %+v, want 2", got)
		}
	})

	t.Run("DeleteAfterCommit deletes the object, then the row, even after a client disconnect", func(t *testing.T) {
		fs := &fakeStore{}
		p := storage.NewPendingDeletion(d, fs, nil)
		key := "slides/post-commit/ok.pdf"
		id := recordInTx(t, p, key, storage.ReasonDeleted)
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		p.DeleteAfterCommit(cctx, id, key)
		if len(fs.deleted) != 1 || fs.deleted[0] != key || fs.ctxErr[0] != nil {
			t.Errorf("deleted = %v, ctxErr = %v", fs.deleted, fs.ctxErr)
		}
		if got := pendingRows(t, d, key); len(got) != 0 {
			t.Errorf("row not removed: %+v", got)
		}
	})

	t.Run("DeleteAfterCommit with MinIO down keeps the row and logs", func(t *testing.T) {
		log, buf := newLogger()
		fs := &fakeStore{deleteErr: errors.New("connection refused")}
		p := storage.NewPendingDeletion(d, fs, log)
		key := "slides/post-commit/down.pdf"
		id := recordInTx(t, p, key, storage.ReasonReplaced)
		p.DeleteAfterCommit(ctx, id, key)
		if got := pendingRows(t, d, key); len(got) != 1 || got[0].Reason != "REPLACED" {
			t.Errorf("rows = %+v, want the REPLACED row kept", got)
		}
		if !strings.Contains(buf.String(), "left for the clean-up job") || !strings.Contains(buf.String(), key) {
			t.Errorf("log = %s", buf.String())
		}
	})

	t.Run("database failures are logged, not panicked", func(t *testing.T) {
		closed := dbtest.Open(t, dsn)
		_ = closed.Close()
		log, buf := newLogger()
		fs := &fakeStore{}
		p := storage.NewPendingDeletion(closed, fs, log)

		if err := p.RecordUploadAborted(ctx, "slides/closed/a.pdf", db.Unknown); err == nil {
			t.Error("RecordUploadAborted on a closed pool: want error")
		}
		p.DeleteAfterCommit(ctx, 1, "slides/closed/b.pdf")
		if len(fs.deleted) != 1 {
			t.Errorf("object delete must still be attempted: %v", fs.deleted)
		}
		for _, want := range []string{"may be orphaned", "row not removed"} {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("log lacks %q: %s", want, buf.String())
			}
		}
	})
}

// Lost-commit cases (Sequence.md §2, §7): InTx through the fault-injecting
// connector, then the compensation a caller makes for the new key.
func TestPendingDeletionAroundLostCommit(t *testing.T) {
	dsn := dbtest.Start(t)
	d := dbtest.Open(t, dsn)
	faulty, faults := dbtest.OpenFaulty(t, dsn)
	ctx := context.Background()
	p := storage.NewPendingDeletion(d, &fakeStore{}, nil)

	for _, tc := range []struct {
		name       string
		fault      dbtest.CommitFault
		want       db.Outcome
		oldKeyRows int // REPLACED rows for the old key: present iff the commit applied
	}{
		{"commit applied, ack lost", dbtest.LostAckCommitted, db.Unknown, 1},
		{"connection lost before commit", dbtest.LostAckNotCommitted, db.Unknown, 0},
		{"deadlock at commit", dbtest.DeadlockAtCommit, db.RolledBack, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldKey := "slides/lost/" + tc.name + "/old.pdf"
			newKey := "slides/lost/" + tc.name + "/new.pdf"
			faults.FailNextCommit(tc.fault)
			out, err := db.InTx(ctx, faulty, func(ctx context.Context, tx *sql.Tx) error {
				_, err := p.RecordInTx(ctx, tx, oldKey, storage.ReasonReplaced)
				return err
			})
			if out != tc.want || err == nil {
				t.Fatalf("InTx = %v, %v; want %v with an error", out, err, tc.want)
			}
			if got := pendingRows(t, d, oldKey); len(got) != tc.oldKeyRows {
				t.Errorf("old key rows = %+v, want %d", got, tc.oldKeyRows)
			}
			// The caller never deletes the new object on Unknown; it records it.
			if err := p.RecordUploadAborted(ctx, newKey, out); err != nil {
				t.Fatal(err)
			}
			got := pendingRows(t, d, newKey)
			if len(got) != 1 || (out == db.Unknown) != (got[0].DueInSec > 0) {
				t.Errorf("new key rows = %+v (outcome %v)", got, out)
			}
		})
	}
	if faults.Fired() != 3 {
		t.Errorf("faults fired = %d, want 3", faults.Fired())
	}

	// The pool stays usable after the injected failures.
	if out, err := db.InTx(ctx, faulty, func(context.Context, *sql.Tx) error { return nil }); out != db.Committed || err != nil {
		t.Errorf("after faults: %v, %v", out, err)
	}
}

// AbortUpload (Sequence.md §2): known rollback deletes now and records only
// when that fails; Unknown never deletes and records with the grace period.
// A cancelled request context does not skip the compensation.
func TestAbortUpload(t *testing.T) {
	d := dbtest.New(t)
	gone, cancel := context.WithCancel(context.Background())
	cancel()

	for _, tc := range []struct {
		name      string
		outcome   db.Outcome
		deleteErr error
		deleted   bool
		rows      int
		graced    bool
	}{
		{"rolled back", db.RolledBack, nil, true, 0, false},
		{"rolled back, delete fails", db.RolledBack, errors.New("minio down"), false, 1, false},
		{"unknown", db.Unknown, nil, false, 1, true},
		{"committed is ignored", db.Committed, nil, false, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := &fakeStore{deleteErr: tc.deleteErr}
			p := storage.NewPendingDeletion(d, fs, nil)
			key := "slides/abort/" + strings.ReplaceAll(tc.name, " ", "-") + ".pdf"
			p.AbortUpload(gone, key, tc.outcome)

			if got := len(fs.deleted) == 1; got != tc.deleted {
				t.Errorf("deleted = %v, want %v", fs.deleted, tc.deleted)
			}
			for _, err := range fs.ctxErr {
				if err != nil {
					t.Errorf("Delete saw ctx error %v; the request's cancellation must be ignored", err)
				}
			}
			if tc.outcome == db.Unknown && len(fs.ctxErr) != 0 {
				t.Error("Unknown must never call Delete")
			}
			got := pendingRows(t, d, key)
			if len(got) != tc.rows {
				t.Fatalf("rows = %+v, want %d", got, tc.rows)
			}
			if tc.rows == 1 {
				if got[0].Reason != string(storage.ReasonUploadAborted) || (got[0].DueInSec > 1) != tc.graced {
					t.Errorf("row = %+v, graced %v", got[0], tc.graced)
				}
			}
		})
	}
}
