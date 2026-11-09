package account

import (
	"strings"
	"unicode/utf8"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Password policy (PRD §6.4). Rule ids are the wire contract (errors[].rule).
const (
	RuleLength        = "LENGTH"
	RuleCharset       = "CHARSET"
	RuleComplexity    = "COMPLEXITY"
	RuleCommon        = "COMMON_PASSWORD"
	RuleKeyboardRun   = "KEYBOARD_RUN"
	RuleSequence      = "SEQUENCE"
	RuleRepeat        = "REPEAT"
	RuleContainsEmpNo = "CONTAINS_EMP_NO"
)

const (
	minPassLen     = 12
	maxPassLen     = 64 // code points
	minPassClasses = 3
	maxRepeat      = 2 // 3+ identical characters in a row is a violation
)

// Rule is one row of the §6.4 table. Violated reports whether password breaks
// the rule; empNo is "" when the submitted employee ID is malformed.
type Rule struct {
	ID       string
	Violated func(password, empNo string) bool
}

// Policy runs its rules in order and reports every violated one. The slice
// order is the PRD §6.4 table order, which is part of the contract
// (testdata/password.json): later PRs add their rule at its table position.
type Policy struct{ Rules []Rule }

// DefaultPolicy returns the rules implemented so far, in table order.
// KEYBOARD_RUN, SEQUENCE and COMMON_PASSWORD arrive with B1b-02/03.
func DefaultPolicy() *Policy {
	return &Policy{Rules: []Rule{
		{RuleLength, violatesLength},
		{RuleCharset, violatesCharset},
		{RuleComplexity, violatesComplexity},
		{RuleRepeat, violatesRepeat},
		{RuleContainsEmpNo, violatesEmpNo},
	}}
}

// Check implements PasswordPolicy. Each violation is a PASSWORD_POLICY error
// on field "password" carrying the rule id.
func (p *Policy) Check(password, empNo string) []web.FieldError {
	var out []web.FieldError
	for _, r := range p.Rules {
		if r.Violated(password, empNo) {
			out = append(out, web.FieldError{Field: fieldPassword, Code: web.CodePasswordPolicy, Rule: r.ID})
		}
	}
	return out
}

func violatesLength(pw, _ string) bool {
	n := utf8.RuneCountInString(pw)
	return n < minPassLen || n > maxPassLen
}

func violatesCharset(pw, _ string) bool {
	for _, r := range pw {
		if r < 0x20 || r > 0x7E { // also catches U+FFFD from invalid UTF-8
			return true
		}
	}
	return false
}

// violatesComplexity counts upper, lower, digit and symbol; a symbol is any
// printable non-alphanumeric ASCII character, space included. Characters
// outside printable ASCII belong to no class (CHARSET reports them).
func violatesComplexity(pw, _ string) bool {
	var upper, lower, digit, symbol bool
	for _, r := range pw {
		switch {
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= '0' && r <= '9':
			digit = true
		case r >= 0x20 && r <= 0x7E:
			symbol = true
		}
	}
	n := 0
	for _, c := range [...]bool{upper, lower, digit, symbol} {
		if c {
			n++
		}
	}
	return n < minPassClasses
}

// violatesRepeat: the same character three or more times in a row, compared
// exactly (case-sensitive: "aAa" is fine).
func violatesRepeat(pw, _ string) bool {
	run, prev := 0, rune(-1)
	for _, r := range pw {
		if r == prev {
			run++
		} else {
			run, prev = 1, r
		}
		if run > maxRepeat {
			return true
		}
	}
	return false
}

func violatesEmpNo(pw, empNo string) bool {
	return empNo != "" && strings.Contains(pw, empNo)
}
