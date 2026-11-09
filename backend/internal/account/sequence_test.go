package account

import "testing"

func TestViolatesSequence(t *testing.T) {
	for _, tc := range []struct {
		pw   string
		want bool
	}{
		{"", false},
		{"abc", false},
		{"abcd", true},
		{"dcba", true},
		{"aBcD", true},
		{"WXYZ", true},
		{"0123", true},
		{"9876", true},
		{"3210", true},
		{"x-abcdefg-y", true}, // longer run, inside other text
		{"xyza", false},       // no wrap
		{"9012", false},       // no wrap
		{"zyxw", true},
		{"abcba", false},  // direction change restarts the count
		{"abbc", false},   // a repeat breaks the run
		{"aaaa", false},   // repeat is not a sequence
		{"89ab", false},   // letters and digits do not join
		{"xyz{", false},   // ASCII neighbour outside the alphabet
		{"/012", false},   // ASCII neighbour outside the alphabet
		{"ab-cd", false},  // a separator breaks the run
		{"abécd", false},  // non-ASCII breaks the run
		{"ab12cd", false}, // alphabets switching break the run
		{"abc0123", true},
	} {
		if got := violatesSequence(tc.pw, ""); got != tc.want {
			t.Errorf("violatesSequence(%q) = %v, want %v", tc.pw, got, tc.want)
		}
	}
}
