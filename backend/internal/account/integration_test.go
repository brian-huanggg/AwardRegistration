//go:build integration

package account_test

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/test/apispec"
)

type env struct {
	db *sql.DB
	h  http.Handler
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := dbtest.New(t)
	auditSvc := audit.New(db, nil)
	errs := server.ErrorAdapter{Audit: auditSvc}
	api := server.API{Account: account.Handler{
		Service: &account.Service{DB: db, Audit: auditSvc, Policy: account.DefaultPolicy()},
		Fail:    errs.Write,
	}}
	h := apispec.Load(t).Validate(t, server.NewPublic(slog.New(slog.DiscardHandler), server.Mount(api)))
	return env{db: db, h: h}
}

func (e env) register(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "it/1")
	req.RemoteAddr = "192.0.2.9:4000"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

const goodPW = "Correct-Horse-1"

func reg(empNo, name, email, pw string) string {
	b, _ := json.Marshal(map[string]string{"empNo": empNo, "name": name, "email": email, "password": pw})
	return string(b)
}

type auditRow struct {
	Action, Outcome, ActorRole, ErrorCode string
	Status                                sql.NullInt64
	Route, Method, Submitted, After       sql.NullString
	TargetType                            sql.NullString
}

func (e env) auditRows(t *testing.T) []auditRow {
	t.Helper()
	rows, err := e.db.Query(`SELECT action, outcome, actor_role, COALESCE(error_code,''), status, route, method,
		submitted_emp_no, CAST(after_json AS CHAR), target_type FROM audit_log ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		if err := rows.Scan(&r.Action, &r.Outcome, &r.ActorRole, &r.ErrorCode, &r.Status, &r.Route, &r.Method,
			&r.Submitted, &r.After, &r.TargetType); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func (e env) users(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM app_user`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct {
		Code   string
		Errors []struct{ Field, Code string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Code
}

func TestRegisterSuccess(t *testing.T) {
	e := newEnv(t)
	rec := e.register(reg("089290", "  王　小明 ", "  Wang.Xiao@Example.COM ", "Correct-Horse-1"))

	if rec.Code != 201 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body) != 3 || body["empNo"] != "089290" || body["name"] != "王 小明" || body["email"] != "wang.xiao@example.com" {
		t.Errorf("body = %v: want the cleaned values, no password, no id", body)
	}
	if rec.Header().Get("Set-Cookie") != "" || rec.Header().Get("Location") != "" {
		t.Errorf("headers = %v: registration sets no cookie and no Location", rec.Header())
	}

	var name, email, hash, role string
	if err := e.db.QueryRow(`SELECT name, email, password_hash, role FROM app_user WHERE emp_no='089290'`).
		Scan(&name, &email, &hash, &role); err != nil {
		t.Fatal(err)
	}
	if name != "王 小明" || email != "wang.xiao@example.com" || role != "USER" {
		t.Errorf("stored %q %q %q", name, email, role)
	}
	if cost, err := bcrypt.Cost([]byte(hash)); err != nil || cost != 10 {
		t.Errorf("hash cost %d err %v", cost, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("Correct-Horse-1")) != nil {
		t.Error("hash does not match the password")
	}

	rows := e.auditRows(t)
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v", rows)
	}
	r := rows[0]
	if r.Action != "AUTH_REGISTER" || r.Outcome != "SUCCESS" || r.ActorRole != "ANONYMOUS" || r.Status.Int64 != 201 ||
		r.Route.String != "/api/v1/auth/register" || r.TargetType.String != "USER" {
		t.Errorf("row = %+v", r)
	}
	if strings.Contains(r.After.String, "@") || strings.Contains(r.After.String, "Correct") ||
		!strings.Contains(r.After.String, "089290") || !strings.Contains(r.After.String, "王 小明") {
		t.Errorf("after_json = %s: want empNo and name, never email or password", r.After.String)
	}
}

func TestRegisterConflicts(t *testing.T) {
	e := newEnv(t)
	if rec := e.register(reg("089290", "A", "a@x.com", goodPW)); rec.Code != 201 {
		t.Fatal(rec.Body)
	}

	cases := []struct {
		name, body, code, reason string
	}{
		{"same employee ID", reg("089290", "B", "b@x.com", goodPW), "EMP_NO_TAKEN", "emp_no_taken"},
		{"same email, any case", reg("089291", "B", " A@X.COM ", goodPW), "EMAIL_TAKEN", "email_taken"},
		{"both taken reports the employee ID", reg("089290", "B", "a@x.com", goodPW), "EMP_NO_TAKEN", "emp_no_taken"},
	}
	for _, tc := range cases {
		rec := e.register(tc.body)
		if rec.Code != 409 || problemCode(t, rec) != tc.code {
			t.Errorf("%s: %d %s", tc.name, rec.Code, rec.Body)
		}
	}
	if n := e.users(t); n != 1 {
		t.Errorf("users = %d", n)
	}

	rows := e.auditRows(t)
	if len(rows) != 4 {
		t.Fatalf("audit rows = %+v", rows)
	}
	for i, tc := range cases {
		r := rows[i+1]
		if r.Action != "AUTH_REGISTER" || r.Outcome != "FAILED" || r.ErrorCode != tc.code || r.Status.Int64 != 409 ||
			!strings.Contains(r.After.String, `"reason":"`+tc.reason+`"`) {
			t.Errorf("%s: row = %+v", tc.name, r)
		}
	}
	if rows[1].Submitted.String != "089290" {
		t.Errorf("submitted = %q", rows[1].Submitted.String)
	}
}

func TestRegisterValidationBeatsConflictAndIsAudited(t *testing.T) {
	e := newEnv(t)
	e.register(reg("089290", "A", "a@x.com", goodPW))

	// The employee ID is taken, but the email is malformed: 422, never 409.
	rec := e.register(reg("089290", "B", "a b@x.com", goodPW))
	if rec.Code != 422 || problemCode(t, rec) != "VALIDATION_FAILED" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.register(reg("12 4", "B/C", "ok@x.com", goodPW))
	if rec.Code != 422 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}

	rows := e.auditRows(t)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	if r := rows[1]; r.Outcome != "FAILED" || r.Status.Int64 != 422 || r.Submitted.String != "089290" ||
		!strings.Contains(r.After.String, `"empNo":"089290"`) || !strings.Contains(r.After.String, `"reason":"validation"`) {
		t.Errorf("422 row = %+v", r)
	}
	// A malformed employee ID is kept in the submitted column only, not in after_json.
	if r := rows[2]; r.Submitted.String != "12 4" || strings.Contains(r.After.String, "empNo") {
		t.Errorf("malformed-ID row = %+v", r)
	}
	if e.users(t) != 1 {
		t.Error("a rejected registration created a user")
	}
}

func TestRegisterMalformedIsAuditedButUnsupportedMediaIsNot(t *testing.T) {
	e := newEnv(t)

	rec := e.register(`{"empNo":"089290"`)
	if rec.Code != 400 || problemCode(t, rec) != "MALFORMED_REQUEST" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader("x"))
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != 415 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}

	rows := e.auditRows(t)
	if len(rows) != 1 || rows[0].Outcome != "FAILED" || rows[0].Status.Int64 != 400 || rows[0].ActorRole != "ANONYMOUS" {
		t.Fatalf("rows = %+v: only the 400 is audited", rows)
	}
}

func TestRegisterRaceHasOneWinner(t *testing.T) {
	e := newEnv(t)
	const n = 6
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.register(reg("089290", "A", "race"+string(rune('a'+i))+"@x.com", goodPW)).Code
		}()
	}
	wg.Wait()

	created, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case 201:
			created++
		case 409:
			conflicts++
		}
	}
	if created != 1 || conflicts != n-1 || e.users(t) != 1 {
		t.Fatalf("codes = %v, users = %d", codes, e.users(t))
	}
}

func TestRegisterWeakPasswordListsEveryRuleInTableOrder(t *testing.T) {
	e := newEnv(t)
	// Format and policy errors come back together; the password contains the
	// employee ID, repeats a character and has only two classes.
	rec := e.register(reg("089290", "A", "nope", "aaa089290"))
	if rec.Code != 422 || problemCode(t, rec) != "VALIDATION_FAILED" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var body struct {
		Errors []struct{ Field, Code, Rule string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range body.Errors {
		got = append(got, f.Field+":"+f.Code+":"+f.Rule)
	}
	want := "email:INVALID_FORMAT: password:PASSWORD_POLICY:LENGTH password:PASSWORD_POLICY:COMPLEXITY " +
		"password:PASSWORD_POLICY:REPEAT password:PASSWORD_POLICY:CONTAINS_EMP_NO"
	if strings.Join(got, " ") != want {
		t.Errorf("errors = %v\nwant     %s", got, want)
	}

	// Alone, a weak password makes PASSWORD_POLICY the top-level code.
	rec = e.register(reg("089290", "A", "a@x.com", "short"))
	if rec.Code != 422 || problemCode(t, rec) != "PASSWORD_POLICY" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if e.users(t) != 0 {
		t.Error("a rejected registration created a user")
	}
}
