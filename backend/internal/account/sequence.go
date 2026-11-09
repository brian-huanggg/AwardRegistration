package account

const minSequence = 4

// seqKey places a character on one of two alphabets: letters a–z (case
// folded) and digits 0–9. ok is false for anything else, which breaks a
// sequence. Letters and digits never join each other ("89ab" is two runs of 2),
// and neighbours in ASCII outside an alphabet do not count ("xyz{" is 3).
func seqKey(r rune) (alpha int8, pos int8, ok bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return 0, int8(r - 'a'), true
	case r >= 'A' && r <= 'Z':
		return 0, int8(r - 'A'), true
	case r >= '0' && r <= '9':
		return 1, int8(r - '0'), true
	}
	return 0, 0, false
}

// violatesSequence: four or more consecutive letters or digits, ascending or
// descending, case-insensitive ("abcd", "DcBa", "4321"). Sequences do not wrap
// ("xyza", "9012" are fine), the same character repeated is not a sequence
// (that is REPEAT), and a run must keep one direction ("abcba" is fine).
// Mirrors violatesKeyboardRun.
func violatesSequence(pw, _ string) bool {
	var prevAlpha, prevPos int8
	have, run, dir := false, 0, int8(0)
	for _, r := range pw {
		a, p, ok := seqKey(r)
		if !ok {
			have, run, dir = false, 0, 0
			continue
		}
		step := int8(0)
		if have && a == prevAlpha {
			step = p - prevPos
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
		prevAlpha, prevPos, have = a, p, true
		if run >= minSequence {
			return true
		}
	}
	return false
}
