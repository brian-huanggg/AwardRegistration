// Command server is the single deployable of the ESG Idea Contest backend.
// Dependencies are wired by hand here (no DI framework).
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	_ "github.com/go-sql-driver/mysql"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.OS()
	if err != nil {
		return err
	}
	log := web.NewLogger(os.Stdout, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("mysql", cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("ESG_DB_DSN: %w", err)
	}
	defer func() { _ = db.Close() }()

	si := server.API{Deadline: deadline.Handler{Store: deadline.SQLStore{DB: db}}}

	var ready atomic.Bool
	return server.Run(ctx, cfg, log, server.NewPublic(log, server.Mount(si)), &ready)
}
