// Package auth owns login, sessions, CSRF and throttling (Backend.md §6).
// This file is the session store (B1-07); the middleware (B1-08) builds on it.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

// Session timeouts (PRD §6.5). Refresh is written at most once per 60 s
// (OI-11), so the effective idle timeout is 29-30 minutes.
const (
	IdleTimeoutSeconds     = 30 * 60
	AbsoluteTimeoutSeconds = 8 * 60 * 60
	RefreshSeconds         = 60

	idBytes = 32
)

var limits = store.Limits{
	IdleSeconds:     IdleTimeoutSeconds,
	AbsoluteSeconds: AbsoluteTimeoutSeconds,
	RefreshSeconds:  RefreshSeconds,
}

// Sessions is the server-side session store. It never sees or stores a raw
// session ID beyond the call that receives it; the table holds SHA-256 only.
type Sessions struct {
	DB *sql.DB
}

// New returns a session store on db.
func New(db *sql.DB) *Sessions { return &Sessions{DB: db} }

// Create issues a new session for the user and returns the ID for the cookie
// (32 random bytes, base64url without padding). Every login calls it, so an ID
// is never reused (no fixation).
func (s *Sessions) Create(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.create(ctx, s.DB, userID)
}

// CreateIn is Create inside the caller's transaction, so login commits the
// session together with its audit row.
func (s *Sessions) CreateIn(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (string, error) {
	return s.create(ctx, tx, userID)
}

func (s *Sessions) create(ctx context.Context, x store.Execer, userID uuid.UUID) (string, error) {
	raw := make([]byte, idBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("session id: %w", err)
	}
	if err := store.Insert(ctx, x, hash(raw), userID); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Validate checks the cookie value and refreshes the session in one atomic
// statement, then returns the caller. ok is false for anything that is not a
// live session of an enabled account: malformed, unknown, idle or absolutely
// expired, disabled user, or removed meanwhile. err is for database failures
// only.
func (s *Sessions) Validate(ctx context.Context, id string) (p security.Principal, ok bool, err error) {
	h, valid := parse(id)
	if !valid {
		return security.Principal{}, false, nil
	}
	if ok, err = store.Touch(ctx, s.DB, h, limits); err != nil || !ok {
		return security.Principal{}, false, err
	}
	return store.Principal(ctx, s.DB, h)
}

// Revoke deletes one session. It is idempotent: an unknown or malformed ID is
// not an error (logout returns 204 either way).
func (s *Sessions) Revoke(ctx context.Context, id string) error {
	h, valid := parse(id)
	if !valid {
		return nil
	}
	return store.Delete(ctx, s.DB, h)
}

// RevokeIn is Revoke inside the caller's transaction.
func (s *Sessions) RevokeIn(ctx context.Context, tx *sql.Tx, id string) error {
	h, valid := parse(id)
	if !valid {
		return nil
	}
	return store.Delete(ctx, tx, h)
}

// RevokeAll deletes every session of the user. It implements
// account.SessionRevoker (wired in main.go; account never imports auth).
func (s *Sessions) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	return store.DeleteByUser(ctx, s.DB, userID)
}

// Binding returns SHA-256 of the session ID, the value the CSRF token is bound
// to (D40). ok is false for a malformed ID.
func Binding(id string) (b []byte, ok bool) { return parse(id) }

// parse decodes a cookie value and returns its SHA-256. Anything that is not
// exactly 32 bytes of base64url is rejected without a database call.
func parse(id string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(raw) != idBytes {
		return nil, false
	}
	return hash(raw), true
}

func hash(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}
