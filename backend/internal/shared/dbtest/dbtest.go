// Package dbtest starts a throwaway MariaDB (testcontainers-go) with the real
// migrations applied, and connects with the production DSN pins. Integration
// tests only; needs Docker.
package dbtest

import (
	"context"
	"database/sql"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go/modules/mariadb"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
)

// Image matches docker-compose.yml and production.
const Image = "mariadb:11.4"

var hostPort = regexp.MustCompile(`@tcp\(([^)]+)\)`)

// DSN returns the production DSN (config.DefaultDSN pins: parseTime, loc=UTC,
// session time_zone, clientFoundRows) pointed at the container's host:port.
func DSN(hostAndPort string) string {
	return hostPort.ReplaceAllString(config.DefaultDSN, "@tcp("+hostAndPort+")")
}

// MigrationFiles lists db/migration/V*.sql in version order.
func MigrationFiles() ([]string, error) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migration")
	files, err := filepath.Glob(filepath.Join(dir, "V*.sql"))
	sort.Strings(files)
	return files, err
}

// New starts MariaDB, applies every migration and returns a pool that uses
// the production DSN. The container is terminated when the test ends.
func New(t testing.TB) *sql.DB {
	t.Helper()
	ctx := context.Background()
	scripts, err := MigrationFiles()
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	ctr, err := mariadb.Run(ctx, Image,
		mariadb.WithDatabase("esg"), mariadb.WithUsername("esg"), mariadb.WithPassword("esg"),
		mariadb.WithScripts(scripts...))
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
	db, err := sql.Open("mysql", DSN(u[1]))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
