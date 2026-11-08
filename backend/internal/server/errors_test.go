package server_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

type fakeRejecter struct{ got []audit.Event }

func (f *fakeRejecter) RecordRejected(_ context.Context, e audit.Event) { f.got = append(f.got, e) }

func serve(t *testing.T, ad server.ErrorAdapter, err error) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/ideas/{id}", func(w http.ResponseWriter, req *http.Request) { ad.Write(w, req, err) })
	req := httptest.NewRequest("GET", "/ideas/123", nil)
	req.RemoteAddr = "192.0.2.7:5555"
	req.Header.Set("User-Agent", "ua/1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAdapterWithReasonRecordsDenied(t *testing.T) {
	id := uuid.New()
	e := web.NewError(web.CodeNotFound, "resource not found")
	e.AuditReason, e.AuditAction = "NOT_FOUND", "IDEA_VIEW"
	e.AuditTargetType, e.AuditTargetID = "IDEA", id
	f := &fakeRejecter{}

	rec := serve(t, server.ErrorAdapter{Audit: f}, fmt.Errorf("wrapped: %w", e))

	if rec.Code != 404 {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(f.got) != 1 {
		t.Fatalf("rows = %d, want 1", len(f.got))
	}
	g := f.got[0]
	if g.Outcome != audit.Denied || g.Action != audit.IdeaView || g.TargetType != audit.TargetIdea || g.TargetID != id ||
		g.Method != "GET" || g.Route != "/ideas/{id}" || g.Status != 404 || g.ErrorCode != "NOT_FOUND" ||
		g.IP.String() != "192.0.2.7" || g.UserAgent != "ua/1" || string(g.After) != `{"reason":"NOT_FOUND"}` {
		t.Fatalf("event = %+v (after %s)", g, g.After)
	}
	if body := rec.Body.String(); contains(body, "reason") {
		t.Fatalf("reason leaked to client: %s", body)
	}
}

func TestAdapterOutcomeOverride(t *testing.T) {
	e := web.NewError(web.CodeLoginThrottled, "")
	e.AuditReason, e.AuditAction, e.AuditOutcome = "KEY_LOCKED", "AUTH_LOGIN", "THROTTLED"
	f := &fakeRejecter{}
	serve(t, server.ErrorAdapter{Audit: f}, e)
	if len(f.got) != 1 || f.got[0].Outcome != audit.Throttled {
		t.Fatalf("got %+v", f.got)
	}
}

func TestAdapterWithoutReasonRecordsNothing(t *testing.T) {
	f := &fakeRejecter{}
	rec := serve(t, server.ErrorAdapter{Audit: f}, web.NewError(web.CodeEmailTaken, "x"))
	if rec.Code != 409 || len(f.got) != 0 {
		t.Fatalf("status %d rows %d", rec.Code, len(f.got))
	}
}

func TestAdapterPlainErrorIs500AndUnaudited(t *testing.T) {
	f := &fakeRejecter{}
	rec := serve(t, server.ErrorAdapter{Audit: f}, errors.New("db password=hunter2"))
	if rec.Code != 500 || len(f.got) != 0 || contains(rec.Body.String(), "hunter2") {
		t.Fatalf("status %d rows %d body %s", rec.Code, len(f.got), rec.Body)
	}
}

func TestAdapterReasonWithoutActionStillAnswers(t *testing.T) {
	e := web.NewError(web.CodeForbidden, "no")
	e.AuditReason = "ROLE"
	f := &fakeRejecter{}
	rec := serve(t, server.ErrorAdapter{Audit: f}, e)
	if rec.Code != 403 || len(f.got) != 0 {
		t.Fatalf("status %d rows %d", rec.Code, len(f.got))
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestAdapterCarriesSubmittedEmpNoAndDetail(t *testing.T) {
	e := web.NewError(web.CodeEmpNoTaken, "taken")
	e.AuditReason, e.AuditAction, e.AuditOutcome = "emp_no_taken", "AUTH_REGISTER", "FAILED"
	e.AuditSubmittedEmpNo = "089290"
	e.AuditDetail = map[string]string{"empNo": "089290", "reason": "must not override"}
	f := &fakeRejecter{}

	rec := serve(t, server.ErrorAdapter{Audit: f}, e)

	if rec.Code != 409 || len(f.got) != 1 {
		t.Fatalf("status %d rows %d", rec.Code, len(f.got))
	}
	g := f.got[0]
	if g.Outcome != audit.Failed || g.SubmittedEmpNo != "089290" || string(g.After) != `{"empNo":"089290","reason":"emp_no_taken"}` {
		t.Fatalf("event = %+v (after %s)", g, g.After)
	}
}
