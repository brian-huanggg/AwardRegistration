package account

import (
	"strings"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/text"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Field limits (PRD §6.3).
const (
	empNoLen     = 6
	maxNameLen   = 50
	maxEmailLen  = 254
	maxPassBytes = 72 // bcrypt silently ignores (x/crypto: rejects) anything longer
)

// Field names as they appear in errors[]; the order is the form order.
const (
	fieldEmpNo    = "empNo"
	fieldName     = "name"
	fieldEmail    = "email"
	fieldPassword = "password"
)

// checkEmpNo: exactly six ASCII digits, nothing is trimmed (PRD §6.3).
// Empty is REQUIRED, anything else wrong is INVALID_FORMAT.
func checkEmpNo(s string) text.Problem {
	if text.Trim(s) == "" {
		return text.Required
	}
	if len(s) != empNoLen {
		return text.InvalidFormat
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return text.InvalidFormat
		}
	}
	return text.None
}

// checkEmail trims, then requires printable ASCII with no whitespace, exactly
// one '@' with non-empty sides and at most 254 characters; the result is
// lower-cased (PRD §6.3, OI-14). Report order: REQUIRED, INVALID_FORMAT,
// OUT_OF_RANGE.
func checkEmail(s string) (string, text.Problem) {
	e := text.Trim(s)
	if e == "" {
		return "", text.Required
	}
	for _, r := range e {
		if r < 0x21 || r > 0x7E { // also rejects whitespace and control characters
			return e, text.InvalidFormat
		}
	}
	local, domain, ok := strings.Cut(e, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") {
		return e, text.InvalidFormat
	}
	if len(e) > maxEmailLen { // ASCII only, so bytes = code points
		return e, text.OutOfRange
	}
	return strings.ToLower(e), text.None
}

// PasswordPolicy checks a candidate password and lists every violation, in
// PRD §6.4 table order. empNo is the submitted employee ID when it is
// well-formed and "" otherwise (the CONTAINS_EMP_NO rule is then skipped).
type PasswordPolicy interface {
	Check(password, empNo string) []web.FieldError
}

// StubPolicy is the placeholder until B1b-01 delivers the §6.4 rules. It only
// refuses what the account cannot store: an empty password and one bcrypt
// cannot hash (longer than 72 bytes). Everything else is accepted.
type StubPolicy struct{}

// Check implements PasswordPolicy.
func (StubPolicy) Check(password, _ string) []web.FieldError {
	switch {
	case password == "":
		return []web.FieldError{{Field: fieldPassword, Code: web.CodeRequired}}
	case len(password) > maxPassBytes:
		return []web.FieldError{{Field: fieldPassword, Code: web.CodeOutOfRange}}
	}
	return nil
}

// topCode is the response code of a 422: the first field error's code when it
// is a specific registered code, else VALIDATION_FAILED (error-codes.md §2).
func topCode(fields []web.FieldError) web.ErrorCode {
	if len(fields) > 0 {
		switch c := fields[0].Code; c {
		case web.CodeInvalidNameChars, web.CodePasswordPolicy:
			return c
		}
	}
	return web.CodeValidationFailed
}
