// Package migrate applies the embedded schema migrations with golang-migrate.
// It is a library; the Job entry point is cmd/migrate.
package migrate

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/brian-huanggg/AwardRegistration/backend/db"
)

// Up applies every pending migration. It returns nil when the schema is already
// current. golang-migrate holds a database lock (GET_LOCK) while it runs, so
// concurrent callers are safe. A dirty version (a migration that failed half
// way) is an error and needs manual repair.
func Up(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("migrate: parse DSN: %w", err)
	}
	cfg.MultiStatements = true // migration files hold several statements
	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("migrate: open: %w", err)
	}
	defer func() { _ = conn.Close() }()

	drv, err := migmysql.WithInstance(conn, &migmysql.Config{DatabaseName: cfg.DBName})
	if err != nil {
		return fmt.Errorf("migrate: driver: %w", err)
	}
	src, err := iofs.New(db.Migrations, "migration")
	if err != nil {
		return fmt.Errorf("migrate: source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}
