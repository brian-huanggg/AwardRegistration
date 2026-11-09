// Package deadline owns the contest's single submission deadline: the derived
// state (PRD 5) and the public GET /deadline endpoint.
package deadline

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// State is the submission state, always derived from the deadline and database time.
type State = api.DeadlineState

// StateOf derives the state: NOT_OPEN without a deadline, OPEN while now is
// before it, CLOSED from the deadline instant on (PRD 5).
func StateOf(now time.Time, deadline *time.Time) State {
	switch {
	case deadline == nil:
		return api.NOTOPEN
	case now.Before(*deadline):
		return api.OPEN
	default:
		return api.CLOSED
	}
}

// Snapshot is the deadline row and the database clock read by one statement.
type Snapshot struct {
	Deadline *time.Time
	Now      time.Time
}

// State derives the state of the snapshot.
func (s Snapshot) State() State { return StateOf(s.Now, s.Deadline) }

// Store reads the deadline.
type Store interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}

// SQLStore reads contest_setting. The row is read without a lock (PRD 5).
type SQLStore struct{ DB *sql.DB }

// Snapshot returns the deadline and NOW(6) from the same statement, so the two
// can never disagree about the clock.
func (s SQLStore) Snapshot(ctx context.Context) (Snapshot, error) {
	var snap Snapshot
	var d sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT deadline, NOW(6) FROM contest_setting WHERE id = 1`).Scan(&d, &snap.Now)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read contest_setting: %w", err)
	}
	if d.Valid {
		snap.Deadline = &d.Time
	}
	return snap, nil
}

// Handler serves GET /deadline and PUT /admin/deadline.
type Handler struct {
	Store  Store
	Setter Setter
	// Fail writes errors (the central adapter, which also records DENIED
	// rows). Nil falls back to a plain problem response.
	Fail func(w http.ResponseWriter, r *http.Request, err error)
}

// body is written by hand: the generated api.Deadline marshals time.Time with
// RFC 3339 nanoseconds, the contract wants truncated milliseconds.
type body struct {
	Deadline   *string `json:"deadline"`
	State      State   `json:"state"`
	ServerTime string  `json:"serverTime"`
}

// GetDeadline answers 200 {deadline, state, serverTime}; a database failure is
// 503 (the endpoint is public and read-only, nothing else can fail).
func (h Handler) GetDeadline(w http.ResponseWriter, r *http.Request) {
	snap, err := h.Store.Snapshot(r.Context())
	if err != nil {
		e := web.NewError(web.CodeServiceUnavailable, "the deadline cannot be read, retry later")
		e.Cause = err
		web.WriteProblem(w, r, e)
		return
	}
	web.WriteJSON(w, http.StatusOK, render(snap))
}

func render(snap Snapshot) body {
	out := body{State: snap.State(), ServerTime: web.FormatTime(snap.Now)}
	if snap.Deadline != nil {
		s := web.FormatTime(*snap.Deadline)
		out.Deadline = &s
	}
	return out
}
