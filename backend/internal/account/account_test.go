package account_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// A Service with no database: every case here must be rejected before any I/O.
func validate(t *testing.T, cmd account.RegisterCommand) *web.Error {
	t.Helper()
	svc := &account.Service{Policy: account.StubPolicy{}}
	_, err := svc.Register(context.Background(), cmd, account.Request{})
	var e *web.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *web.Error, got %v", err)
	}
	return e
}

func good() account.RegisterCommand {
	return account.RegisterCommand{EmpNo: "089290", Name: "王小明", Email: "a@b.c", Password: "whatever-goes"}
}

func codes(e *web.Error) []string {
	var out []string
	for _, f := range e.Fields {
		out = append(out, f.Field+":"+string(f.Code))
	}
	return out
}

func TestRegisterValidation(t *testing.T) {
	cases := []struct {
		name   string
		mut    func(*account.RegisterCommand)
		top    web.ErrorCode
		fields string
	}{
		{"empty emp no", func(c *account.RegisterCommand) { c.EmpNo = " " }, web.CodeValidationFailed, "empNo:REQUIRED"},
		{"emp no 5 digits", func(c *account.RegisterCommand) { c.EmpNo = "12345" }, web.CodeValidationFailed, "empNo:INVALID_FORMAT"},
		{"emp no 7 digits", func(c *account.RegisterCommand) { c.EmpNo = "1234567" }, web.CodeValidationFailed, "empNo:INVALID_FORMAT"},
		{"emp no letter", func(c *account.RegisterCommand) { c.EmpNo = "12345a" }, web.CodeValidationFailed, "empNo:INVALID_FORMAT"},
		{"emp no not trimmed", func(c *account.RegisterCommand) { c.EmpNo = " 12345" }, web.CodeValidationFailed, "empNo:INVALID_FORMAT"},
		{"emp no fullwidth digits", func(c *account.RegisterCommand) { c.EmpNo = "１２３４５６" }, web.CodeValidationFailed, "empNo:INVALID_FORMAT"},
		{"name empty after clean-up", func(c *account.RegisterCommand) { c.Name = " 　\t" }, web.CodeValidationFailed, "name:REQUIRED"},
		{"name forbidden char", func(c *account.RegisterCommand) { c.Name = "a/b" }, web.CodeInvalidNameChars, "name:INVALID_NAME_CHARS"},
		{"name control char", func(c *account.RegisterCommand) { c.Name = "a\x00b" }, web.CodeValidationFailed, "name:INVALID_FORMAT"},
		{"name 51 code points", func(c *account.RegisterCommand) { c.Name = strings.Repeat("名", 51) }, web.CodeValidationFailed, "name:OUT_OF_RANGE"},
		{"email with space", func(c *account.RegisterCommand) { c.Email = "a b@x.com" }, web.CodeValidationFailed, "email:INVALID_FORMAT"},
		{"email without @", func(c *account.RegisterCommand) { c.Email = "ab.com" }, web.CodeValidationFailed, "email:INVALID_FORMAT"},
		{"email two @", func(c *account.RegisterCommand) { c.Email = "a@b@c" }, web.CodeValidationFailed, "email:INVALID_FORMAT"},
		{"email empty local", func(c *account.RegisterCommand) { c.Email = "@b" }, web.CodeValidationFailed, "email:INVALID_FORMAT"},
		{"email empty domain", func(c *account.RegisterCommand) { c.Email = "a@" }, web.CodeValidationFailed, "email:INVALID_FORMAT"},
		{"email non-ASCII", func(c *account.RegisterCommand) { c.Email = "é@x.com" }, web.CodeValidationFailed, "email:INVALID_FORMAT"},
		{"email empty", func(c *account.RegisterCommand) { c.Email = "" }, web.CodeValidationFailed, "email:REQUIRED"},
		{"email 255 chars", func(c *account.RegisterCommand) { c.Email = strings.Repeat("a", 253) + "@b" }, web.CodeValidationFailed, "email:OUT_OF_RANGE"},
		{"password empty", func(c *account.RegisterCommand) { c.Password = "" }, web.CodeValidationFailed, "password:REQUIRED"},
		{"password beyond bcrypt", func(c *account.RegisterCommand) { c.Password = strings.Repeat("p", 73) }, web.CodeValidationFailed, "password:OUT_OF_RANGE"},
		{"all fields in form order", func(c *account.RegisterCommand) {
			c.EmpNo, c.Name, c.Email, c.Password = "x", "a:b", "nope", ""
		}, web.CodeValidationFailed, "empNo:INVALID_FORMAT name:INVALID_NAME_CHARS email:INVALID_FORMAT password:REQUIRED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := good()
			tc.mut(&cmd)
			e := validate(t, cmd)
			if e.Status != 422 || e.Code != tc.top {
				t.Errorf("status %d code %s, want 422 %s", e.Status, e.Code, tc.top)
			}
			if got := strings.Join(codes(e), " "); got != tc.fields {
				t.Errorf("fields = %q, want %q", got, tc.fields)
			}
			// Every failure is an AUTH_REGISTER FAILED attempt for the adapter.
			if e.AuditAction != "AUTH_REGISTER" || e.AuditOutcome != "FAILED" || e.AuditReason != "validation" {
				t.Errorf("audit fields = %q %q %q", e.AuditAction, e.AuditOutcome, e.AuditReason)
			}
		})
	}
}

func TestFailedAuditCarriesEmpNoOnlyWhenWellFormed(t *testing.T) {
	cmd := good()
	cmd.Name = ""
	e := validate(t, cmd)
	if e.AuditSubmittedEmpNo != "089290" || e.AuditDetail["empNo"] != "089290" {
		t.Errorf("well-formed: submitted %q detail %v", e.AuditSubmittedEmpNo, e.AuditDetail)
	}
	cmd.EmpNo = "12x"
	e = validate(t, cmd)
	if e.AuditSubmittedEmpNo != "12x" || e.AuditDetail != nil {
		t.Errorf("malformed: submitted %q detail %v", e.AuditSubmittedEmpNo, e.AuditDetail)
	}
}

type spyPolicy struct{ empNo string }

func (s *spyPolicy) Check(_, empNo string) []web.FieldError {
	s.empNo = empNo
	return []web.FieldError{{Field: "password", Code: web.CodePasswordPolicy}}
}

func TestPolicyRunsWithFormatErrorsAndSeesEmpNoOnlyWhenWellFormed(t *testing.T) {
	spy := &spyPolicy{}
	svc := &account.Service{Policy: spy}
	cmd := good()
	cmd.Email = "bad"

	_, err := svc.Register(context.Background(), cmd, account.Request{})
	var e *web.Error
	if !errors.As(err, &e) {
		t.Fatal(err)
	}
	if got := strings.Join(codes(e), " "); got != "email:INVALID_FORMAT password:PASSWORD_POLICY" {
		t.Errorf("fields = %q: format and policy errors must come together", got)
	}
	if spy.empNo != "089290" {
		t.Errorf("policy got empNo %q", spy.empNo)
	}

	cmd.EmpNo = "bad"
	_, _ = svc.Register(context.Background(), cmd, account.Request{})
	if spy.empNo != "" {
		t.Errorf("policy got empNo %q for a malformed ID; CONTAINS_EMP_NO must be skipped", spy.empNo)
	}
}

func TestTopCodeIsPasswordPolicyWhenFirst(t *testing.T) {
	svc := &account.Service{Policy: &spyPolicy{}}
	_, err := svc.Register(context.Background(), good(), account.Request{})
	var e *web.Error
	if !errors.As(err, &e) || e.Code != web.CodePasswordPolicy || e.Status != 422 {
		t.Fatalf("%v", err)
	}
}

func handlerFor(got *error) http.Handler {
	h := account.Handler{
		Service: &account.Service{Policy: account.StubPolicy{}},
		Fail:    func(_ http.ResponseWriter, _ *http.Request, err error) { *got = err },
	}
	return http.HandlerFunc(h.Register)
}

func post(h http.Handler, contentType, body string) {
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestHandlerTransportErrors(t *testing.T) {
	cases := []struct {
		name, ct, body string
		code           web.ErrorCode
		audited        bool
	}{
		{"broken JSON", "application/json", `{`, web.CodeMalformedRequest, true},
		{"missing field", "application/json", `{"empNo":"089290","name":"n","email":"a@b"}`, web.CodeMalformedRequest, true},
		{"null field", "application/json", `{"empNo":"089290","name":"n","email":"a@b","password":null}`, web.CodeMalformedRequest, true},
		{"wrong type", "application/json", `{"empNo":89290,"name":"n","email":"a@b","password":"x"}`, web.CodeMalformedRequest, true},
		{"duplicate key", "application/json", `{"empNo":"1","empNo":"2","name":"n","email":"a@b","password":"x"}`, web.CodeMalformedRequest, true},
		{"not JSON", "text/plain", `{}`, web.CodeUnsupportedMedia, false},
		{"no content type", "", `{}`, web.CodeUnsupportedMedia, false},
		{"over 16 KiB", "application/json", `{"name":"` + strings.Repeat("x", 17<<10) + `"}`, web.CodeRequestTooLarge, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got error
			post(handlerFor(&got), tc.ct, tc.body)
			var e *web.Error
			if !errors.As(got, &e) || e.Code != tc.code {
				t.Fatalf("got %v, want %s", got, tc.code)
			}
			if audited := e.AuditReason != ""; audited != tc.audited {
				t.Errorf("audited = %v, want %v (PRD 6.11: only 400, 409, 422)", audited, tc.audited)
			}
			if tc.audited && (e.AuditAction != "AUTH_REGISTER" || e.AuditOutcome != "FAILED" || e.AuditSubmittedEmpNo != "") {
				t.Errorf("audit fields = %q %q %q", e.AuditAction, e.AuditOutcome, e.AuditSubmittedEmpNo)
			}
		})
	}
}
