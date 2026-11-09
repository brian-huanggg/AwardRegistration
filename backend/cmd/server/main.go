// Command server is the single deployable of the ESG Idea Contest backend.
// Dependencies are wired by hand here (no DI framework).
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	_ "github.com/go-sql-driver/mysql"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/auth"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/config"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/job"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
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

	auditSvc := audit.New(db, log)
	errs := server.ErrorAdapter{Audit: auditSvc, Log: log}
	csrf, err := auth.NewCSRF(cfg.CSRFKey, cfg.CSRFKeyPrevious)
	if err != nil {
		return err
	}
	sessions := auth.New(db)
	logins, err := auth.NewLogins(sessions, auditSvc, auth.DBThrottle{DB: db})
	if err != nil {
		return err
	}
	slides, err := storage.NewMinIO(storage.MinIOConfig{
		Endpoint: cfg.MinIO.Endpoint, AccessKey: cfg.MinIO.AccessKey, SecretKey: cfg.MinIO.SecretKey,
		Bucket: cfg.MinIO.Bucket, Secure: cfg.MinIO.Secure,
	})
	if err != nil {
		return err
	}
	pending := storage.NewPendingDeletion(db, slides, log)
	si := server.API{
		Auth:    auth.Handler{CSRF: csrf, Log: log},
		Logins:  auth.LoginHandler{Logins: logins, Fail: errs.Write},
		Logouts: auth.LogoutHandler{Logouts: &auth.Logouts{Sessions: sessions, Audit: auditSvc}, Fail: errs.Write},
		Me:      auth.MeHandler{Sessions: sessions, Fail: errs.Write},
		Account: account.Handler{
			Service: &account.Service{DB: db, Audit: auditSvc, Policy: account.DefaultPolicy()},
			Fail:    errs.Write,
		},
		Deadline: deadline.Handler{Store: deadline.SQLStore{DB: db}},
		Ideas: idea.Handler{
			Service: &idea.Service{
				DB: db, Audit: auditSvc,
				Gate:     deadline.Gate{Store: deadline.SQLStore{DB: db}},
				Slides:   server.IdeaSlideRules(),
				Objects:  slides,
				Aborter:  pending,
				Replaced: server.IdeaReplacedKeys{P: pending},
				Deleted:  server.IdeaDeletedKeys{P: pending},
			},
			ExpectedFilename: storage.ExpectedFilenameBase,
			Fail:             errs.Write,
			Log:              log,
		},
	}

	// Background jobs stop when the root context is cancelled, or when Run
	// returns for another reason, and are awaited before the pool closes.
	jobCtx, stopJobs := context.WithCancel(ctx)
	var jobs sync.WaitGroup
	defer func() { stopJobs(); jobs.Wait() }()
	jobs.Go(func() {
		job.Run(jobCtx, log, job.Spec{Name: "auth-purge", Interval: auth.PurgeInterval, Jitter: auth.PurgeJitter},
			auth.Purger{DB: db, Log: log}.Purge)
	})

	// Readiness also waits for the first successful Admin read (OI-40).
	checks := []server.Check{server.DBCheck(db)}
	if cfg.AdminFile != "" {
		sync := &account.AdminSync{
			DB: db, Dir: account.FileAdminDirectory{Path: cfg.AdminFile}, Audit: auditSvc, Revoker: sessions, Log: log,
		}
		checks = append(checks, sync.ReadyCheck)
		jobs.Go(func() {
			job.Run(jobCtx, log, job.Spec{Name: "admin-sync", Interval: account.AdminSyncInterval, Jitter: account.AdminSyncJitter},
				sync.Run)
		})
	}

	var ready atomic.Bool
	return server.Run(ctx, cfg, log, server.NewPublic(log, server.Mount(si),
		web.TrustedProxies(cfg.ProxyHops),
		auth.Middleware(sessions, server.IsPublic, log),
		csrf.Middleware(errs.Write, server.ActionFor)), &ready, checks...)
}
