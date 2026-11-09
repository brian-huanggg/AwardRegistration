package account

import "testing"

func TestViolatesKeyboardRun(t *testing.T) {
	for _, tc := range []struct {
		pw   string
		want bool
	}{
		{"", false},
		{"qwe", false},
		{"qwer", true},
		{"rewq", true},
		{"QwEr", true},
		{"1234", true},
		{"4321", true},
		{"!@#$", true},
		{"1@3$", true},
		{"=-09", true},
		{"[poi", true},
		{"zxcv", true},
		{"/.,m", true},
		{"asdf", true},
		{"';lk", true},
		{"{POI", true},
		{"xqwertyx", true},
		{"p-asdf", true}, // rows do not join: only asdf counts
		{"pasd", false},  // no wrap row to row
		{"0-=q", false},  // "0-=" is 3 keys, then a new row
		{"qwewq", false}, // direction change restarts the count
		{"qw~er", false},
		{"qwdf", false},
		{"qqqq", false},
		{"1111", false},
		{"`123", false},
		{"éqwe", false},
		{"aaa-sdf", false},
	} {
		if got := violatesKeyboardRun(tc.pw, ""); got != tc.want {
			t.Errorf("violatesKeyboardRun(%q) = %v, want %v", tc.pw, got, tc.want)
		}
	}
}
