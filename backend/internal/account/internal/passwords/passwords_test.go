package passwords

import "testing"

func TestIsCommon(t *testing.T) {
	for _, tc := range []struct {
		pw   string
		want bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"PassWord", true},
		{"123456", true},
		{"films+pic+galeries", true},
		{"Films+Pic+Galeries", true},
		{"eyphed", true}, // last line of the file
		{"", false},
		{" password", false}, // nothing is trimmed
		{"password ", false},
		{"password1!", false}, // not on the list
		{"xpasswordx", false}, // an entry inside a longer password does not match
		{"Correct-Horse-1", false},
	} {
		if got := IsCommon(tc.pw); got != tc.want {
			t.Errorf("IsCommon(%q) = %v, want %v", tc.pw, got, tc.want)
		}
	}
}

func TestListIsLoaded(t *testing.T) {
	// 10,001 lines, no case-folded duplicates (NOTICE).
	if got := Len(); got != 10001 {
		t.Errorf("Len() = %d, want 10001", got)
	}
}
