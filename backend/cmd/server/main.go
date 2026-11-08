// Command server is the single deployable of the ESG Idea Contest backend.
// Dependencies are wired by hand here (no DI framework).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

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

	var ready atomic.Bool
	return server.Run(ctx, cfg, log, server.NewPublic(log, nil), &ready)
}
