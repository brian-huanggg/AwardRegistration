// Package dbtest starts a throwaway MariaDB (testcontainers-go) with the real
// migrations applied, and connects with the production DSN pins. Integration
// tests only; needs Docker.
package dbtest

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mariadb"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/migrate"
)

// Image matches docker-compose.yml and production.
const Image = "mariadb:11.4"

var hostPort = regexp.MustCompile(`@tcp\(([^)]+)\)`)

// DSN returns the production DSN (config.DefaultDSN pins: parseTime, loc=UTC,
// session time_zone, clientFoundRows) pointed at the container's host:port.
func DSN(hostAndPort string) string {
	return hostPort.ReplaceAllString(config.DefaultDSN, "@tcp("+hostAndPort+")")
}

// New starts MariaDB, applies every migration and returns a pool that uses
// the production DSN. The container is terminated when the test ends.
func New(t testing.TB) *sql.DB {
	t.Helper()
	return Open(t, Start(t))
}

// Start starts MariaDB with every migration applied and returns the production
// DSN for it. opts customise the container (for example a Docker network, so a
// Toxiproxy container can reach it). The container is terminated when the test
// ends.
func Start(t testing.TB, opts ...testcontainers.ContainerCustomizer) string {
	t.Helper()
	ctx := context.Background()
	opts = append([]testcontainers.ContainerCustomizer{
		mariadb.WithDatabase("esg"), mariadb.WithUsername("esg"), mariadb.WithPassword("esg"),
	}, opts...)
	ctr, err := mariadb.Run(ctx, Image, opts...)
	if err != nil {
		t.Fatalf("start mariadb: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.WithoutCancel(ctx)) })

	cs, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	u := hostPort.FindStringSubmatch(cs)
	if u == nil {
		t.Fatalf("unexpected connection string %q", cs)
	}
	dsn := DSN(u[1])
	if err := migrate.Up(dsn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return dsn
}

// Open opens a pool on dsn, closed when the test ends.
func Open(t testing.TB, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
