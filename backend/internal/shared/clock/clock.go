// Package clock is the one time authority: the database clock (Architecture
// principle 4). Application code never calls time.Now for a business decision.
package clock

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Clock returns the current instant.
type Clock interface {
	Now(ctx context.Context) (time.Time, error)
}

// DbClock reads SELECT NOW(6). With the pinned DSN (loc=UTC, time_zone +00:00)
// the result is a UTC time.Time with microsecond precision.
type DbClock struct{ DB *sql.DB }

// Now returns the database time.
func (c DbClock) Now(ctx context.Context) (time.Time, error) {
	var t time.Time
	if err := c.DB.QueryRowContext(ctx, `SELECT NOW(6)`).Scan(&t); err != nil {
		return time.Time{}, fmt.Errorf("read database clock: %w", err)
	}
	return t, nil
}

// Fixed is a Clock for unit tests that must not touch a database.
type Fixed time.Time

// Now returns the fixed instant.
func (f Fixed) Now(context.Context) (time.Time, error) { return time.Time(f), nil }
