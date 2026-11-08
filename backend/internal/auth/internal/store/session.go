// Package store holds the SQL of the auth module. Only the auth module may
// import it (Go's internal rule); no other module touches the session table.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

// Limits are the session timeouts in seconds (PRD §6.5).
type Limits struct {
	IdleSeconds     int
	AbsoluteSeconds int
	RefreshSeconds  int
}

// Execer is *sql.DB or *sql.Tx.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Insert stores a new session row. hash is SHA-256 of the session ID.
func Insert(ctx context.Context, x Execer, hash []byte, userID uuid.UUID) error {
	_, err := x.ExecContext(ctx,
		`INSERT INTO session (id_hash, user_id) VALUES (?, ?)`, hash, userID.String())
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// Touch validates and refreshes a session in ONE statement on database time.
// It reports true when the row matched: the session exists, is inside both
// timeouts and its account is enabled. RowsAffected counts matched rows
// (clientFoundRows=true in the DSN), so a row that needs no refresh still
// counts; a test locks this. last_access_at is written only when older than
// the refresh interval (OI-11).
func Touch(ctx context.Context, db *sql.DB, hash []byte, l Limits) (bool, error) {
	res, err := db.ExecContext(ctx,
		`UPDATE session s JOIN app_user u ON u.id = s.user_id
		    SET s.last_access_at = IF(s.last_access_at < NOW(6) - INTERVAL ? SECOND, NOW(6), s.last_access_at)
		  WHERE s.id_hash = ?
		    AND u.enabled
		    AND s.last_access_at > NOW(6) - INTERVAL ? SECOND
		    AND s.created_at     > NOW(6) - INTERVAL ? SECOND`,
		l.RefreshSeconds, hash, l.IdleSeconds, l.AbsoluteSeconds)
	if err != nil {
		return false, fmt.Errorf("touch session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("touch session rows: %w", err)
	}
	return n == 1, nil
}

// Principal reads the caller behind a session. The role comes from app_user,
// never from the session. found is false when the session vanished between
// Touch and this read (logout, revoke, purge).
func Principal(ctx context.Context, db *sql.DB, hash []byte) (p security.Principal, found bool, err error) {
	var id, role string
	err = db.QueryRowContext(ctx,
		`SELECT u.id, u.emp_no, u.role
		   FROM session s JOIN app_user u ON u.id = s.user_id
		  WHERE s.id_hash = ? AND u.enabled`, hash).Scan(&id, &p.EmpNo, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return security.Principal{}, false, nil
	}
	if err != nil {
		return security.Principal{}, false, fmt.Errorf("read session principal: %w", err)
	}
	if p.UserID, err = uuid.Parse(id); err != nil {
		return security.Principal{}, false, fmt.Errorf("parse user id %q: %w", id, err)
	}
	p.Role = security.Role(role)
	return p, true, nil
}

// Delete removes one session; deleting a missing one is not an error.
func Delete(ctx context.Context, db *sql.DB, hash []byte) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM session WHERE id_hash = ?`, hash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteByUser removes every session of one account.
func DeleteByUser(ctx context.Context, db *sql.DB, userID uuid.UUID) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM session WHERE user_id = ?`, userID.String()); err != nil {
		return fmt.Errorf("delete sessions of user: %w", err)
	}
	return nil
}

// DeleteExpired removes sessions past either timeout and returns how many.
func DeleteExpired(ctx context.Context, db *sql.DB, l Limits) (int64, error) {
	res, err := db.ExecContext(ctx,
		`DELETE FROM session
		  WHERE last_access_at <= NOW(6) - INTERVAL ? SECOND
		     OR created_at     <= NOW(6) - INTERVAL ? SECOND`,
		l.IdleSeconds, l.AbsoluteSeconds)
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge sessions rows: %w", err)
	}
	return n, nil
}

// Account is the login view of app_user: what is needed to check a password
// and answer the login response. PasswordHash never leaves the auth module.
type Account struct {
	ID           uuid.UUID
	EmpNo        string
	Name         string
	Email        *string // NULL for Admin
	PasswordHash string
	Role         security.Role
	Enabled      bool
}

// AccountByEmpNo reads one account by employee ID; found is false when none
// exists.
func AccountByEmpNo(ctx context.Context, db *sql.DB, empNo string) (a Account, found bool, err error) {
	var id, role string
	err = db.QueryRowContext(ctx,
		`SELECT id, emp_no, name, email, password_hash, role, enabled
		   FROM app_user WHERE emp_no = ?`, empNo).
		Scan(&id, &a.EmpNo, &a.Name, &a.Email, &a.PasswordHash, &role, &a.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, fmt.Errorf("read account: %w", err)
	}
	if a.ID, err = uuid.Parse(id); err != nil {
		return Account{}, false, fmt.Errorf("parse user id %q: %w", id, err)
	}
	a.Role = security.Role(role)
	return a, true, nil
}
