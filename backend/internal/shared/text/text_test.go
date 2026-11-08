package text

import "testing"

func TestIsWhitespaceMatchesTwentyFiveCodePoints(t *testing.T) {
	n := 0
	for r := rune(0); r <= 0x10FFFF; r++ {
		if IsWhitespace(r) {
			n++
		}
	}
	if n != 25 {
		t.Fatalf("White_Space count = %d, want 25", n)
	}
	for _, r := range []rune{0x200B, 0xFEFF, 0x180E, 0x001C} {
		if IsWhitespace(r) {
			t.Errorf("U+%04X must not be whitespace", r)
		}
	}
}
