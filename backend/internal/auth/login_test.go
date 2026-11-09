package auth

import (
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func median(ds []time.Duration) time.Duration {
	slices.Sort(ds)
	return ds[len(ds)/2]
}

func timeCompare(hash []byte, pw string) time.Duration {
	const n = 7
	ds := make([]time.Duration, n)
	for i := range ds {
		start := time.Now()
		comparePassword(hash, pw)
		ds[i] = time.Since(start)
	}
	return median(ds)
}

// Unknown vs wrong password (B1-10 acceptance): one cost-10 comparison either
// way, so the medians are close. The bound is loose on purpose: bcrypt takes
// tens of milliseconds, a skipped comparison would be ~1000x off.
func TestComparePasswordTimingSimilar(t *testing.T) {
	l, err := NewLogins(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	real, err := bcrypt.GenerateFromPassword([]byte("Correct-Horse-1"), bcryptCost)
	if err != nil {
		t.Fatal(err)
	}
	wrong := timeCompare(real, "Wrong-Horse-99")
	dummy := timeCompare(l.dummyHash, "Wrong-Horse-99")
	long := timeCompare(l.dummyHash, strings.Repeat("x", 100)) // > 72 bytes

	for name, d := range map[string]time.Duration{"dummy": dummy, "over 72 bytes": long} {
		if d < wrong/2 || d > wrong*2 {
			t.Errorf("%s comparison %v vs wrong-password %v: not similar (a comparison was skipped?)", name, d, wrong)
		}
	}
}

func TestComparePassword(t *testing.T) {
	pw72 := strings.Repeat("a", 72)
	h72, _ := bcrypt.GenerateFromPassword([]byte(pw72), bcryptCost)
	tests := []struct {
		name, pw string
		want     bool
	}{
		{"exact 72 bytes", pw72, true},
		{"73 bytes never matches, even with a matching 72-byte prefix", pw72 + "b", false},
		{"wrong", "nope", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		if got := comparePassword(h72, tc.pw); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestLockKey(t *testing.T) {
	tests := []struct{ in, want string }{
		{"123456", "123456"},
		{"12\x00\n34\t56\x7f", "123456"},
		{strings.Repeat("a", 40), strings.Repeat("a", 32)},
		{strings.Repeat("工", 33), strings.Repeat("工", 32)},
		{"", ""},
	}
	for _, tc := range tests {
		if got := LockKey(tc.in); got != tc.want {
			t.Errorf("LockKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
