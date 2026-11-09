// Command migrate applies the schema migrations and exits. It is the entry
// point of the migration Job (Argo CD PreSync) and of `make migrate`. The
// server never migrates on its own.
package main

import (
	"fmt"
	"os"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/migrate"
)

func main() {
	dsn := os.Getenv("ESG_DB_DSN")
	if dsn == "" {
		dsn = config.DefaultDSN
	}
	if err := migrate.Up(dsn); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	fmt.Println("migrations up to date")
}
