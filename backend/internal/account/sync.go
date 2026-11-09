package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

// Admin sync schedule (PRD §4.2): every 60 s. The jitter keeps replicas out of
// step; the work is safe on several replicas at once either way.
const (
	AdminSyncInterval = 60 * time.Second
	AdminSyncJitter   = 10 * time.Second
)

// Skip reasons recorded in the ADMIN_SYNC FAILED row and the log. Like the
// Reject* constants they are labels, never user text.
const (
	SkipUserConflict = "USER_CONFLICT" // emp_no belongs to a self-registered User
	SkipWrite        = "WRITE_FAILED"  // the database refused this one entry
)

// ErrNotReady is returned by AdminSync.ReadyCheck until the first complete sync
// has succeeded (OI-40).
var ErrNotReady = errors.New("admin sync: first read has not succeeded yet")

// SessionRevoker ends every session of one account. auth.Sessions implements
// it; main.go passes it in, so account never imports auth (D30).
type SessionRevoker interface {
	RevokeAll(ctx context.Context, userID uuid.UUID) error
}

// AdminRecorder writes ADMIN_SYNC rows. *audit.Service implements it.
type AdminRecorder interface {
	Recorder
	RecordRejected(ctx context.Context, e audit.Event)
}

// AdminSync makes the Admin rows of app_user follow the AdminDirectory
// (PRD §4.2, OI-40). Run it with job.Run on every replica; no leader is
// elected.
//
// Several replicas may sync at once. Each Admin is handled in its own
// transaction that locks the row (SELECT … FOR UPDATE) and compares it with the
// directory entry, so a replica that finds nothing to change writes nothing,
// and only the replica whose write changed a row writes the ADMIN_SYNC row.
//
// Rules, each per entry so one bad entry never stops the others:
//   - unknown emp_no: insert an enabled ADMIN (no email);
//   - existing ADMIN: update name and hash when they differ, and re-enable a
//     disabled one;
//   - emp_no of a USER: skip, log, count, ADMIN_SYNC FAILED; never converted;
//   - ADMIN missing from the directory: disable (never delete). An entry that
//     the directory rejected (bad cost, …) does not count as missing;
//   - directory unreadable: change nothing, count a failure, keep the Admins.
//
// A hash change or a disable also revokes that Admin's sessions (PRD §4.2).
// The revoke runs after the commit by the replica whose write changed the row.
// A disabled account is refused by session validation anyway; a failed revoke
// after a hash change is kept in memory and retried at the start of each run.
type AdminSync struct {
	DB      *sql.DB
	Dir     AdminDirectory
	Audit   AdminRecorder
	Revoker SessionRevoker
	Log     *slog.Logger // nil discards

	ready    atomic.Bool
	failures atomic.Uint64
	skipped  atomic.Uint64

	mu        sync.Mutex
	revokeDue map[uuid.UUID]struct{} // revokes that failed, retried next run
	reported  map[string]struct{}    // skips already audited, so a standing conflict is not re-audited every 60 s
}

// Ready reports whether the first complete sync has succeeded. It never goes
// back to false: a later failure keeps the existing Admins (OI-40).
func (s *AdminSync) Ready() bool { return s.ready.Load() }

// ReadyCheck is the readiness probe check: ErrNotReady until Ready.
func (s *AdminSync) ReadyCheck(context.Context) error {
	if !s.ready.Load() {
		return ErrNotReady
	}
	return nil
}

// Failures counts runs that could not read the directory or reach the
// database. It backs the alert metric of OI-40.
func (s *AdminSync) Failures() uint64 { return s.failures.Load() }

// Skipped counts entries skipped by a run (directory rejects, User conflicts,
// refused writes), once per run they are seen.
func (s *AdminSync) Skipped() uint64 { return s.skipped.Load() }

// errNoChange makes db.InTx roll back a transaction that found nothing to do.
var errNoChange = errors.New("admin sync: no change")

type existing struct {
	id      uuid.UUID
	role    string
	name    string
	hash    string
	enabled bool
}

// Run is one sync pass, the function handed to job.Run.
func (s *AdminSync) Run(ctx context.Context) error {
	log := s.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	ctx = security.WithSystem(ctx)
	s.retryRevokes(ctx)

	snap, err := s.Dir.List(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.failures.Add(1)
			log.ErrorContext(ctx, "admin sync: directory unreadable; Admins kept", "error", err.Error())
		}
		return err
	}

	seen := make(map[string]struct{})
	skip := func(empNo, reason string) {
		s.skipped.Add(1)
		key := reason + "/" + empNo
		seen[key] = struct{}{}
		log.ErrorContext(ctx, "admin sync: entry skipped", "empNo", empNo, "reason", reason)
		s.mu.Lock()
		_, dup := s.reported[key]
		s.mu.Unlock()
		if !dup {
			s.Audit.RecordRejected(ctx, failedEvent(empNo, reason))
		}
	}

	known := make(map[string]struct{}, len(snap.Entries)+len(snap.Rejected))
	for _, r := range snap.Rejected {
		known[r.EmpNo] = struct{}{}
		skip(r.EmpNo, r.Reason)
	}
	var infra error
	for _, e := range snap.Entries {
		known[e.EmpNo] = struct{}{}
		switch err := s.apply(ctx, e); {
		case err == nil, errors.Is(err, errNoChange):
		case errors.Is(err, errUserConflict):
			skip(e.EmpNo, SkipUserConflict)
		case isEntryRefused(err):
			log.ErrorContext(ctx, "admin sync: write refused", "empNo", e.EmpNo, "error", err.Error())
			skip(e.EmpNo, SkipWrite)
		default:
			infra = errors.Join(infra, fmt.Errorf("admin %s: %w", e.EmpNo, err))
		}
	}
	if infra == nil {
		infra = s.disableMissing(ctx, known)
	}
	if infra != nil {
		if ctx.Err() == nil {
			s.failures.Add(1)
			log.ErrorContext(ctx, "admin sync: database error; Admins kept", "error", infra.Error())
		}
		return infra
	}

	s.mu.Lock()
	s.reported = seen
	s.mu.Unlock()
	s.ready.Store(true)
	return nil
}

var errUserConflict = errors.New("admin sync: emp_no belongs to a User")

// apply brings one directory entry into app_user.
func (s *AdminSync) apply(ctx context.Context, e AdminEntry) error {
	var revoke uuid.UUID
	_, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		revoke = uuid.Nil
		cur, found, err := lockByEmpNo(ctx, tx, e.EmpNo)
		if err != nil {
			return err
		}
		if !found {
			id := uuid.New()
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app_user (id, emp_no, name, email, password_hash, role, enabled)
				 VALUES (?, ?, ?, NULL, ?, 'ADMIN', TRUE)`,
				id.String(), e.EmpNo, e.Name, e.Hash); err != nil {
				return fmt.Errorf("insert admin: %w", err)
			}
			return s.record(ctx, tx, id, e.EmpNo, map[string]any{"change": "added", "name": e.Name})
		}
		if cur.role != "ADMIN" {
			return errUserConflict
		}
		nameChanged, hashChanged, reenabled := cur.name != e.Name, cur.hash != e.Hash, !cur.enabled
		if !nameChanged && !hashChanged && !reenabled {
			return errNoChange
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE app_user SET name = ?, password_hash = ?, enabled = TRUE WHERE id = ? AND role = 'ADMIN'`,
			e.Name, e.Hash, cur.id.String())
		if err != nil {
			return fmt.Errorf("update admin: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errNoChange
		}
		after := map[string]any{"change": "updated"}
		if reenabled {
			after["change"] = "re-enabled"
		}
		if nameChanged {
			after["name"] = e.Name
		}
		if hashChanged {
			after["hashChanged"] = true // the hash itself is never logged
			after["sessionsRevoked"] = true
			revoke = cur.id
		}
		return s.record(ctx, tx, cur.id, e.EmpNo, after)
	})
	if err == nil && revoke != uuid.Nil {
		s.revoke(ctx, revoke)
	}
	if key, dup := db.DuplicateKey(err); dup && strings.Contains(key, "emp_no") {
		// Another replica inserted it between our SELECT and INSERT; it
		// wrote the audit row.
		return errNoChange
	}
	return err
}

// isEntryRefused reports a table CHECK failure: this entry is bad, the rest of
// the directory is fine.
func isEntryRefused(err error) bool { return db.IsCheckViolation(err) }

// disableMissing disables every enabled ADMIN that the directory neither lists
// nor rejected.
func (s *AdminSync) disableMissing(ctx context.Context, known map[string]struct{}) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT emp_no FROM app_user WHERE role = 'ADMIN' AND enabled`)
	if err != nil {
		return fmt.Errorf("list admins: %w", err)
	}
	var gone []string
	for rows.Next() {
		var empNo string
		if err := rows.Scan(&empNo); err != nil {
			_ = rows.Close()
			return fmt.Errorf("list admins: %w", err)
		}
		if _, ok := known[empNo]; !ok {
			gone = append(gone, empNo)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("list admins: %w", err)
	}

	var errs error
	for _, empNo := range gone {
		var revoke uuid.UUID
		_, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
			cur, found, err := lockByEmpNo(ctx, tx, empNo)
			if err != nil {
				return err
			}
			revoke = uuid.Nil
			if !found || cur.role != "ADMIN" || !cur.enabled {
				return errNoChange // another replica was first
			}
			if _, err := tx.ExecContext(ctx, `UPDATE app_user SET enabled = FALSE WHERE id = ?`, cur.id.String()); err != nil {
				return fmt.Errorf("disable admin: %w", err)
			}
			revoke = cur.id
			return s.record(ctx, tx, cur.id, empNo, map[string]any{"change": "disabled", "sessionsRevoked": true})
		})
		if err == nil && revoke != uuid.Nil {
			s.revoke(ctx, revoke)
		}
		if err != nil && !errors.Is(err, errNoChange) {
			errs = errors.Join(errs, fmt.Errorf("disable admin %s: %w", empNo, err))
		}
	}
	return errs
}

func lockByEmpNo(ctx context.Context, tx *sql.Tx, empNo string) (cur existing, found bool, err error) {
	var id string
	err = tx.QueryRowContext(ctx,
		`SELECT id, role, name, password_hash, enabled FROM app_user WHERE emp_no = ? FOR UPDATE`, empNo).
		Scan(&id, &cur.role, &cur.name, &cur.hash, &cur.enabled)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return existing{}, false, nil
	case err != nil:
		return existing{}, false, fmt.Errorf("select app_user: %w", err)
	}
	if cur.id, err = uuid.Parse(id); err != nil {
		return existing{}, false, fmt.Errorf("select app_user: id: %w", err)
	}
	return cur, true, nil
}

func (s *AdminSync) record(ctx context.Context, tx *sql.Tx, id uuid.UUID, empNo string, after map[string]any) error {
	after["empNo"] = empNo
	b, _ := json.Marshal(after)
	return s.Audit.Record(ctx, tx, audit.Event{
		Action: audit.AdminSync, Outcome: audit.Success,
		ActorRole:  security.ActorSystem,
		TargetType: audit.TargetUser, TargetID: id,
		After: b,
	})
}

func failedEvent(empNo, reason string) audit.Event {
	b, _ := json.Marshal(map[string]string{"empNo": empNo, "reason": reason})
	return audit.Event{
		Action: audit.AdminSync, Outcome: audit.Failed,
		ActorRole: security.ActorSystem,
		ErrorCode: reason,
		After:     b,
	}
}

// revoke ends the sessions of id; a failure is logged and queued for retry.
func (s *AdminSync) revoke(ctx context.Context, id uuid.UUID) {
	if s.Revoker == nil {
		return
	}
	if err := s.Revoker.RevokeAll(ctx, id); err != nil {
		if ctx.Err() == nil {
			s.logger().ErrorContext(ctx, "admin sync: session revoke failed; will retry", "userId", id.String(), "error", err.Error())
		}
		s.mu.Lock()
		if s.revokeDue == nil {
			s.revokeDue = make(map[uuid.UUID]struct{})
		}
		s.revokeDue[id] = struct{}{}
		s.mu.Unlock()
	}
}

func (s *AdminSync) retryRevokes(ctx context.Context) {
	s.mu.Lock()
	due := s.revokeDue
	s.revokeDue = nil
	s.mu.Unlock()
	for id := range due {
		s.revoke(ctx, id)
	}
}

func (s *AdminSync) logger() *slog.Logger {
	if s.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.Log
}
