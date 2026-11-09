//go:build integration

package idea_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

func TestGetIdeaHandler(t *testing.T) {
	d := dbtest.New(t)
	leader := addUser(t, d, "100001", "USER")
	other := addUser(t, d, "100002", "USER")
	admin := addUser(t, d, "900001", "ADMIN")
	n := newIdea(leader.UserID, "Idea A")
	if err := insert(d, n); err != nil {
		t.Fatal(err)
	}
	aud := audit.New(d, nil)
	errs := server.ErrorAdapter{Audit: aud}
	h := idea.Handler{Service: &idea.Service{DB: d, Audit: aud}, ExpectedFilename: expectedBase, Fail: errs.Write}

	call := func(p security.Principal, id uuid.UUID) *httptest.ResponseRecorder {
		r := chi.NewRouter()
		r.Get("/ideas/{id}", func(w http.ResponseWriter, req *http.Request) { h.GetIdea(w, req, api.IdeaId(id)) })
		req := httptest.NewRequest(http.MethodGet, "/ideas/"+id.String(), nil)
		req = req.WithContext(security.WithPrincipal(context.Background(), p))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	count := func(where string) (c int) {
		if err := d.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE ` + where).Scan(&c); err != nil {
			t.Fatal(err)
		}
		return c
	}

	// Leader: 200, pendingRequests is [] (not null), expected filename.
	rec := call(leader, n.ID)
	if rec.Code != 200 {
		t.Fatalf("leader: %d %s", rec.Code, rec.Body)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["expectedFilenameBase"] != "I_BSID_User 100001_100001_Idea A" ||
		body["slides"].(map[string]any)["downloadFilename"] != "I_BSID_User 100001_100001_Idea A.pdf" {
		t.Errorf("filename: %v", body)
	}
	if pr, ok := body["pendingRequests"].([]any); !ok || len(pr) != 0 {
		t.Errorf("pendingRequests = %v", body["pendingRequests"])
	}
	if count(`action = 'IDEA_VIEW'`) != 0 {
		t.Error("a Leader view must not write IDEA_VIEW")
	}

	// Admin: 200 and one IDEA_VIEW SUCCESS row.
	if rec := call(admin, n.ID); rec.Code != 200 {
		t.Fatalf("admin: %d", rec.Code)
	}
	if count(`action = 'IDEA_VIEW' AND outcome = 'SUCCESS' AND actor_role = 'ADMIN'`) != 1 {
		t.Error("admin view: want one IDEA_VIEW SUCCESS row")
	}

	// Hidden and missing: identical 404 (bar the request ID), both audited DENIED.
	hidden, missing := call(other, n.ID), call(other, uuid.New())
	if hidden.Code != 404 || missing.Code != 404 {
		t.Fatalf("codes %d %d", hidden.Code, missing.Code)
	}
	strip := func(rr *httptest.ResponseRecorder) (map[string]any, http.Header) {
		var m map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &m)
		delete(m, "requestId")
		return m, rr.Header()
	}
	hb, hh := strip(hidden)
	mb, mh := strip(missing)
	if len(hb) != len(mb) || hb["code"] != mb["code"] || hb["detail"] != mb["detail"] {
		t.Errorf("bodies differ: %v vs %v", hb, mb)
	}
	for k := range hh {
		if hh.Get(k) != mh.Get(k) {
			t.Errorf("header %s differs: %q vs %q", k, hh.Get(k), mh.Get(k))
		}
	}
	if got := count(`action = 'IDEA_VIEW' AND outcome = 'DENIED' AND status = 404 AND error_code = 'NOT_FOUND'`); got != 2 {
		t.Errorf("DENIED rows = %d, want 2", got)
	}
}

// expectedBase mirrors storage.ExpectedFilenameBase (idea must not import storage).
func expectedBase(category, org, leaderName, leaderEmpNo, ideaName string) (string, error) {
	return "I_" + org + "_" + leaderName + "_" + leaderEmpNo + "_" + ideaName, nil
}
