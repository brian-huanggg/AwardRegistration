package team

import (
	"errors"
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func TestValidate(t *testing.T) {
	name, err := validate("100001", "  My \t Idea\n")
	if err != nil || name != "My Idea" {
		t.Fatalf("clean: %q %v", name, err)
	}
	for _, tc := range []struct {
		emp, name string
		code      web.ErrorCode
		fields    []string
	}{
		{"", "x", web.CodeValidationFailed, []string{"leaderEmpNo"}},
		{"12345", "x", web.CodeValidationFailed, []string{"leaderEmpNo"}},
		{"12345a", "x", web.CodeValidationFailed, []string{"leaderEmpNo"}},
		{" 10001", "x", web.CodeValidationFailed, []string{"leaderEmpNo"}},
		{"100001", " \t", web.CodeValidationFailed, []string{"ideaName"}},
		{"100001", strings.Repeat("a", 76), web.CodeValidationFailed, []string{"ideaName"}},
		{"100001", "a/b", web.CodeInvalidNameChars, []string{"ideaName"}},
		{"x", "", web.CodeValidationFailed, []string{"leaderEmpNo", "ideaName"}},
	} {
		_, err := validate(tc.emp, tc.name)
		var e *web.Error
		if !errors.As(err, &e) || e.Code != tc.code || len(e.Fields) != len(tc.fields) {
			t.Errorf("%q/%q: %v", tc.emp, tc.name, err)
			continue
		}
		for i, f := range tc.fields {
			if e.Fields[i].Field != f {
				t.Errorf("%q/%q: field %d = %s, want %s", tc.emp, tc.name, i, e.Fields[i].Field, f)
			}
		}
	}
}
