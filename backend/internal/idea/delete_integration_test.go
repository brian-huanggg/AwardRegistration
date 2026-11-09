//go:build integration

package idea_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

func (e *createEnv) del(p security.Principal, id uuid.UUID) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Delete("/ideas/{id}", func(w http.ResponseWriter, req *http.Request) {
		e.h.DeleteIdea(w, req, uuid.MustParse(chi.URLParam(req, "id")))
	})
	req := httptest.NewRequest(http.MethodDelete, "/ideas/"+id.String(), nil)
	req = req.WithContext(security.WithPrincipal(context.Background(), p))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestDeleteIdeaSuccess(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, key := e.createdIdea(t, leader, "Idea A")

	if rec := e.del(leader, id); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM idea WHERE id = ?`, id.String()); n != 0 {
		t.Errorf("idea rows %d, want 0", n)
	}
	if _, ok := e.objects.puts[key]; ok {
		t.Error("object still stored")
	}
	if n := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion`); n != 0 {
		t.Errorf("pending rows %d, want 0", n)
	}
	var before string
	if err := e.db.QueryRow(`SELECT before_json FROM audit_log WHERE action = 'IDEA_DELETE' AND outcome = 'SUCCESS' AND target_id = ?`,
		id.String()).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, `"name":"Idea A"`) || !strings.Contains(before, `"ext":"pdf"`) {
		t.Errorf("audit before=%s", before)
	}
	// The Idea is gone for everyone: same uniform 404.
	if rec := e.del(leader, id); rec.Code != 404 {
		t.Errorf("second delete: %d", rec.Code)
	}
}

// MinIO down after the commit: the delete still succeeds and the key stays
// pending as DELETED for the clean-up job.
func TestDeleteIdeaStorageDownKeepsPendingRow(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, key := e.createdIdea(t, leader, "Idea A")
	e.objects.deleteErr = fmt.Errorf("minio down")

	if rec := e.del(leader, id); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion WHERE object_key = ? AND reason = 'DELETED'`, key); n != 1 {
		t.Errorf("DELETED rows: %d, want 1", n)
	}
}

func TestDeleteIdeaRefusals(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	other := addUser(t, e.db, "100002", "USER")
	admin := addUser(t, e.db, "900001", "ADMIN")
	id, _ := e.createdIdea(t, leader, "Idea A")

	if rec := e.del(other, id); rec.Code != 404 {
		t.Errorf("stranger: %d", rec.Code)
	}
	if rec := e.del(leader, uuid.New()); rec.Code != 404 {
		t.Errorf("missing: %d", rec.Code)
	}
	e.setDeadline(t, "NOW(6) - INTERVAL 1 SECOND")
	if rec := e.del(leader, id); rec.Code != 403 || decodeProblem(t, rec).Code != "SUBMISSION_CLOSED" {
		t.Errorf("leader after deadline: %d %s", rec.Code, rec.Body)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_DELETE' AND outcome = 'DENIED'`); n != 3 {
		t.Errorf("DENIED rows %d, want 3", n)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM idea`); n != 1 {
		t.Fatalf("refusals changed the data: %d ideas", n)
	}
	// Admin is exempt from the deadline.
	if rec := e.del(admin, id); rec.Code != http.StatusNoContent {
		t.Errorf("admin after deadline: %d %s", rec.Code, rec.Body)
	}
}
