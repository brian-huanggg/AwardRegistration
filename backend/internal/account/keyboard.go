package account

import "strings"

const minKeyboardRun = 4

// keyboardRows are the four rows of US QWERTY (PRD §6.4). Rows do not join
// and do not wrap: the end of one row is not adjacent to the start of the next.
var keyboardRows = [...]string{
	"1234567890-=",
	"qwertyuiop[]",
	"asdfghjkl;'",
	"zxcvbnm,./",
}

// shiftedRows holds the shifted glyph of each key, position for position with
// keyboardRows (letters are folded by case instead).
var shiftedRows = [...]string{
	"!@#$%^&*()_+",
	"QWERTYUIOP{}",
	"ASDFGHJKL:\"",
	"ZXCVBNM<>?",
}

// keyPos is a key's row and column on the keyboard.
type keyPos struct{ row, col int8 }

// keyOf maps every unshifted and shifted glyph to its key. Glyphs on no row
// (` ~ \ | and anything non-ASCII) are absent and break a run.
var keyOf = func() map[rune]keyPos {
	m := make(map[rune]keyPos, 2*len(keyboardRows[0])*4)
	for r := range keyboardRows {
		for c, g := range keyboardRows[r] {
			m[g] = keyPos{int8(r), int8(c)}
		}
		for c, g := range shiftedRows[r] {
			m[g] = keyPos{int8(r), int8(c)}
		}
	}
	return m
}()

// violatesKeyboardRun: four or more adjacent keys on one row, read left to
// right or right to left, case-insensitive, a shifted glyph counting as its
// key (`!@#$` is `1234`). The same key repeated is not a run (that is REPEAT),
// and a run must keep one direction ("qwer" yes, "qwqw" no).
func violatesKeyboardRun(pw, _ string) bool {
	var prev keyPos
	have, run, dir := false, 0, int8(0)
	for _, r := range strings.ToLower(pw) {
		k, ok := keyOf[r]
		if !ok {
			have, run, dir = false, 0, 0
			continue
		}
		step := int8(0)
		if have && k.row == prev.row {
			step = k.col - prev.col
		}
		if step == 1 || step == -1 {
			if run >= 2 && step == dir {
				run++
			} else {
				run, dir = 2, step
			}
		} else {
			run, dir = 1, 0
		}
		prev, have = k, true
		if run >= minKeyboardRun {
			return true
		}
	}
	return false
}
