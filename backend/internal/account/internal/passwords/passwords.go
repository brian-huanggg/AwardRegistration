// Package passwords holds the bundled common-password blocklist (PRD §6.4,
// rule COMMON_PASSWORD). Source and licence of the list: NOTICE in this
// directory. The frontend lazy-loads the same file for its live checklist.
package passwords

import (
	_ "embed"
	"strings"
)

//go:embed common-passwords.txt
var commonList string

// common holds every list entry folded to lower case. The list is printable
// ASCII, so strings.ToLower is an exact ASCII case fold here.
var common = func() map[string]struct{} {
	lines := strings.Split(commonList, "\n")
	m := make(map[string]struct{}, len(lines))
	for _, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		if l != "" {
			m[strings.ToLower(l)] = struct{}{}
		}
	}
	return m
}()

// IsCommon reports whether pw equals a list entry, ignoring case. The whole
// password is compared as submitted: nothing is trimmed, and a list entry
// inside a longer password does not match.
func IsCommon(pw string) bool {
	_, ok := common[strings.ToLower(pw)]
	return ok
}

// Len is the number of distinct entries (after case folding).
func Len() int { return len(common) }
