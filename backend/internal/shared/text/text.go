// Package text implements the shared clean-up and counting rules of PRD §6.2.
// The frontend implements the same rules; both run testdata/text-cleanup.json.
package text

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Problem is the first validation error of one field, in PRD §6.2 order.
// The values equal the API error codes; the empty Problem means valid.
type Problem string

const (
	None             Problem = ""
	Required         Problem = "REQUIRED"
	InvalidFormat    Problem = "INVALID_FORMAT"
	InvalidNameChars Problem = "INVALID_NAME_CHARS"
	OutOfRange       Problem = "OUT_OF_RANGE"
)

// forbiddenNameChars become part of the expected filename (PRD §6.2 rule 8).
const forbiddenNameChars = `/\:*?"<>|`

// IsWhitespace reports the Unicode White_Space property (25 code points).
// It is an explicit table on purpose: strings.TrimSpace and unicode.IsSpace
// are not allowed (PRD §6.2 rule 2).
func IsWhitespace(r rune) bool {
	switch {
	case r >= 0x0009 && r <= 0x000D,
		r == 0x0020, r == 0x0085, r == 0x00A0, r == 0x1680,
		r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// Trim removes leading and trailing White_Space code points.
func Trim(s string) string { return strings.TrimFunc(s, IsWhitespace) }

// Count returns the length in Unicode code points.
func Count(s string) int { return utf8.RuneCountInString(s) }

// CleanName cleans a person name or Idea name: NFC, trim, then every
// whitespace run becomes one ASCII space.
func CleanName(s string) string {
	s = Trim(norm.NFC.String(s))
	var b strings.Builder
	inRun := false
	for _, r := range s {
		if IsWhitespace(r) {
			inRun = true
			continue
		}
		if inRun {
			b.WriteByte(' ')
			inRun = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// CleanSummary cleans a summary: NFC, CRLF and CR become LF, then trim.
func CleanSummary(s string) string {
	s = norm.NFC.String(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return Trim(s)
}

// CheckName cleans s as a name and reports its first problem; max is the
// inclusive code-point limit (minimum is 1, which is the REQUIRED rule).
func CheckName(s string, max int) (string, Problem) {
	c := CleanName(s)
	return c, check(c, max, true)
}

// CheckSummary cleans s as a summary and reports its first problem.
func CheckSummary(s string, max int) (string, Problem) {
	c := CleanSummary(s)
	return c, check(c, max, false)
}

func check(c string, max int, name bool) Problem {
	if c == "" {
		return Required
	}
	for _, r := range c {
		if unicode.Is(unicode.Cc, r) && (name || r != '\n') {
			return InvalidFormat
		}
	}
	if name && strings.ContainsAny(c, forbiddenNameChars) {
		return InvalidNameChars
	}
	if Count(c) > max {
		return OutOfRange
	}
	return None
}
