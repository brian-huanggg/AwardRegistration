//go:build integration

package idea_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
)

// put sends PUT /ideas/{id} with a raw body.
func (e *createEnv) put(t *testing.T, p security.Principal, id uuid.UUID, ct, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Put("/ideas/{id}", func(w http.ResponseWriter, req *http.Request) {
		e.h.UpdateIdea(w, req, uuid.MustParse(chi.URLParam(req, "id")))
	})
	req := httptest.NewRequest(http.MethodPut, "/ideas/"+id.String(), strings.NewReader(body))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	req = req.WithContext(security.WithPrincipal(context.Background(), p))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

const editJSON = `{"category":"OUTSTANDING_IMPACT","org":"ICSD","direction":"GREEN_MANUFACTURING","name":"Idea B","summary":"new summary"}`

func (e *createEnv) seed(t *testing.T, leader security.Principal, name string) uuid.UUID {
	t.Helper()
	n := newIdea(leader.UserID, name)
	if err := insert(e.db, n); err != nil {
		t.Fatal(err)
	}
	return n.ID
}

func TestUpdateIdeaSuccess(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id := e.seed(t, leader, "Idea A")

	rec := e.put(t, leader, id, "application/json", editJSON)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	for _, want := range []string{`"name":"Idea B"`, `"org":"ICSD"`, `"version":1`,
		`"expectedFilenameBase":"O_ICSD_User 100001_100001_Idea B"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body lacks %s: %s", want, rec.Body)
		}
	}
	// Every field differs from the seed, so every field is in the audit documents.
	var before, after string
	if err := e.db.QueryRow(`SELECT before_json, after_json FROM audit_log WHERE action = 'IDEA_UPDATE' AND outcome = 'SUCCESS' AND target_id = ?`,
		id.String()).Scan(&before, &after); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, `"name":"Idea A"`) || !strings.Contains(after, `"name":"Idea B"`) {
		t.Errorf("audit before=%s after=%s", before, after)
	}
}

func TestUpdateIdeaAuditsChangedFieldsOnly(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id := e.seed(t, leader, "Idea A")
	// Only the summary differs from the seed (COOL_IDEA/BSID/TALENT_DEVELOPMENT/"Idea A").
	body := `{"category":"COOL_IDEA","org":"BSID","direction":"TALENT_DEVELOPMENT","name":"Idea A","summary":"changed"}`
	if rec := e.put(t, leader, id, "application/json", body); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var before, after string
	if err := e.db.QueryRow(`SELECT before_json, after_json FROM audit_log WHERE action = 'IDEA_UPDATE' AND target_id = ?`,
		id.String()).Scan(&before, &after); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, `"summary":"s"`) || !strings.Contains(after, `"summary":"changed"`) ||
		strings.Contains(before, "name") || strings.Contains(after, "name") {
		t.Errorf("audit before=%s after=%s", before, after)
	}
}

func TestUpdateIdeaNoChangeKeepsVersion(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id := e.seed(t, leader, "Idea A")
	same := `{"category":"COOL_IDEA","org":"BSID","direction":"TALENT_DEVELOPMENT","name":"Idea A","summary":"s"}`
	rec := e.put(t, leader, id, "application/json", same)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":0`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_UPDATE' AND outcome = 'SUCCESS'`) != 1 {
		t.Error("a no-change edit still writes one IDEA_UPDATE row")
	}
}

func TestUpdateIdeaRefusedBeforeBody(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	other := addUser(t, e.db, "100002", "USER")
	admin := addUser(t, e.db, "900001", "ADMIN")
	id := e.seed(t, leader, "Idea A")
	broken := "{not json" // would be 400 if the body were read

	check := func(label string, p security.Principal, target uuid.UUID, status int, code string) {
		t.Helper()
		rec := e.put(t, p, target, "application/json", broken)
		if rec.Code != status || decodeProblem(t, rec).Code != code {
			t.Fatalf("%s: %d %s", label, rec.Code, rec.Body)
		}
	}
	check("hidden", other, id, 404, "NOT_FOUND")
	check("missing", leader, uuid.New(), 404, "NOT_FOUND")
	check("admin", admin, id, 403, "FORBIDDEN")
	e.setDeadline(t, "NULL")
	check("not open", leader, id, 403, "SUBMISSION_NOT_OPEN")
	e.setDeadline(t, "NOW(6) - INTERVAL 1 SECOND")
	check("closed", leader, id, 403, "SUBMISSION_CLOSED")

	reasons := func(r string) int {
		return e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_UPDATE' AND outcome = 'DENIED'
		                    AND JSON_VALUE(after_json, '$.reason') = ?`, r)
	}
	if reasons("NOT_FOUND") != 2 || reasons("ROLE") != 1 || reasons("STATE") != 2 {
		t.Error("DENIED reasons: want 2 NOT_FOUND, 1 ROLE, 2 STATE")
	}
	var name string
	if err := e.db.QueryRow(`SELECT name FROM idea WHERE id = ?`, id.String()).Scan(&name); err != nil || name != "Idea A" {
		t.Errorf("a refused edit changed the row: %q, %v", name, err)
	}
}

func TestUpdateIdeaBodyErrors(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id := e.seed(t, leader, "Idea A")
	tests := []struct {
		label, ct, body string
		status          int
		code            string
	}{
		{"not json type", "text/plain", editJSON, 415, "UNSUPPORTED_MEDIA_TYPE"},
		{"broken", "application/json", "{", 400, "MALFORMED_REQUEST"},
		{"missing field", "application/json", `{"category":"COOL_IDEA"}`, 400, "MALFORMED_REQUEST"},
		{"wrong type", "application/json", strings.Replace(editJSON, `"Idea B"`, `5`, 1), 400, "MALFORMED_REQUEST"},
		{"too large", "application/json", `{"summary":"` + strings.Repeat("x", 17<<10) + `"}`, 413, "REQUEST_TOO_LARGE"},
		{"empty name", "application/json", strings.Replace(editJSON, `"Idea B"`, `"  "`, 1), 422, "VALIDATION_FAILED"},
		{"bad enum", "application/json", strings.Replace(editJSON, `"ICSD"`, `"XXXX"`, 1), 422, "VALIDATION_FAILED"},
		{"name chars", "application/json", strings.Replace(editJSON, `"Idea B"`, `"a/b"`, 1), 422, "INVALID_NAME_CHARS"},
		{"filename too long", "application/json", strings.Replace(editJSON, `"Idea B"`, `"`+strings.Repeat("字", 75)+`"`, 1), 422, "FILENAME_TOO_LONG"},
	}
	for _, tc := range tests {
		rec := e.put(t, leader, id, tc.ct, tc.body)
		if rec.Code != tc.status || decodeProblem(t, rec).Code != tc.code {
			t.Errorf("%s: %d %s", tc.label, rec.Code, rec.Body)
		}
	}
	var version int
	if err := e.db.QueryRow(`SELECT version FROM idea WHERE id = ?`, id.String()).Scan(&version); err != nil || version != 0 {
		t.Errorf("rejected edits changed the row: version %d, %v", version, err)
	}
}

func TestUpdateIdeaDuplicate(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	other := addUser(t, e.db, "100002", "USER")
	e.seed(t, leader, "Idea B")
	id := e.seed(t, leader, "Idea A")
	otherID := e.seed(t, other, "Idea A")

	rec := e.put(t, leader, id, "application/json", editJSON) // renames to "Idea B", which leader owns
	if rec.Code != 409 || decodeProblem(t, rec).Code != "IDEA_DUPLICATE" {
		t.Fatalf("duplicate: %d %s", rec.Code, rec.Body)
	}
	// The same name under another Leader is fine (names are unique per Leader).
	if rec := e.put(t, other, otherID, "application/json", editJSON); rec.Code != 200 {
		t.Fatalf("other leader: %d %s", rec.Code, rec.Body)
	}
}

func TestUpdateIdeaConcurrentSerializes(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id := e.seed(t, leader, "Idea A")
	const n = 8
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.Replace(editJSON, `"Idea B"`, `"Idea `+strings.Repeat("x", i+1)+`"`, 1)
			if rec := e.put(t, leader, id, "application/json", body); rec.Code != 200 {
				t.Errorf("edit %d: %d %s", i, rec.Code, rec.Body)
			}
		}()
	}
	wg.Wait()
	var version int
	if err := e.db.QueryRow(`SELECT version FROM idea WHERE id = ?`, id.String()).Scan(&version); err != nil || version != n {
		t.Errorf("version %d, want %d (%v)", version, n, err)
	}
	if got := e.count(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'IDEA_UPDATE' AND outcome = 'SUCCESS'`); got != n {
		t.Errorf("audit rows %d, want %d", got, n)
	}
}
