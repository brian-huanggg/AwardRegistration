// Package store holds the SQL of the account module. Only the account module
// may import it (Go's internal rule); no other module touches app_user.
package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// NewUser is a row to insert. Every value is already validated and cleaned;
// the table's CHECKs are the second line of defence (V003).
type NewUser struct {
	ID           uuid.UUID
	EmpNo        string
	Name         string
	Email        string // lower-cased
	PasswordHash string // bcrypt, cost 10
}

// InsertUser inserts a USER in the caller's transaction. A unique-key
// violation (emp_no or email) is returned as the driver error; the service
// maps it with db.DuplicateKey.
func InsertUser(ctx context.Context, tx *sql.Tx, u NewUser) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO app_user (id, emp_no, name, email, password_hash, role)
		 VALUES (?, ?, ?, ?, ?, 'USER')`,
		u.ID.String(), u.EmpNo, u.Name, u.Email, u.PasswordHash)
	if err != nil {
		return fmt.Errorf("insert app_user: %w", err)
	}
	return nil
}
