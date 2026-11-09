//go:build integration

package account_test

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/test/apispec"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestRegisterLogsNeverContainPassword drives register (created, duplicate,
// weak password, malformed JSON) at DEBUG level and asserts the submitted
// password never reaches a log line (PRD §6.5 / §8.2).
func TestRegisterLogsNeverContainPassword(t *testing.T) {
	buf := &syncBuf{}
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	db := dbtest.New(t)
	auditSvc := audit.New(db, log)
	errs := server.ErrorAdapter{Audit: auditSvc, Log: log}
	api := server.API{Account: account.Handler{
		Service: &account.Service{DB: db, Audit: auditSvc, Policy: account.DefaultPolicy()},
		Fail:    errs.Write,
	}}
	e := env{db: db, h: apispec.Load(t).Validate(t, server.NewPublic(log, server.Mount(api)))}

	const (
		pw1 = "Sup3r-Secret-Horse-1"
		pw2 = "An0ther-Secret-Horse-2"
		pw3 = "Dup-Secret-Horse-3"
	)
	e.register(reg("089290", "Wang", "wang@example.com", pw1))                                  // 201
	e.register(reg("089290", "Wang", "wang2@example.com", pw2))                                 // duplicate
	e.register(reg("089291", "Lee", "lee@example.com", "short"))                                // weak
	e.register(`{"empNo":"089292","name":"X","email":"x@example.com","password":"` + pw3 + `"`) // malformed

	out := buf.String()
	if strings.TrimSpace(out) == "" {
		t.Fatal("no log output captured; the test would prove nothing")
	}
	for _, s := range []string{pw1, pw2, pw3} {
		if strings.Contains(out, s) {
			t.Errorf("log contains password %q", s)
		}
	}
}
