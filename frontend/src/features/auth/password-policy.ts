/**
 * Password policy of PRD §6.4, ported rule for rule from the backend
 * (backend/internal/account/password.go, keyboard.go, sequence.go) and checked
 * against the same cases (testdata/password.json). The server stays the
 * authority; this drives the live checklist on the register page.
 */

/** Rule ids in PRD §6.4 table order; they are the wire contract (`errors[].rule`). */
export const PASSWORD_RULES = [
  "LENGTH",
  "CHARSET",
  "COMPLEXITY",
  "COMMON_PASSWORD",
  "KEYBOARD_RUN",
  "SEQUENCE",
  "REPEAT",
  "CONTAINS_EMP_NO",
] as const;
export type PasswordRule = (typeof PASSWORD_RULES)[number];

/** Whole-password blocklist lookup (case-insensitive); see `common-passwords.ts`. */
export type CommonCheck = (password: string) => boolean;

const MIN_LEN = 12;
const MAX_LEN = 64; // code points
const MIN_CLASSES = 3;
const MAX_REPEAT = 2; // 3+ identical characters in a row is a violation
const MIN_KEYBOARD_RUN = 4;
const MIN_SEQUENCE = 4;

/** The password as code points (a JS string is UTF-16; the server counts code points). */
const codePoints = (s: string): number[] => Array.from(s, (c) => c.codePointAt(0)!);

/**
 * Lower-cases one code point exactly as Go's `strings.ToLower` does for every
 * result that matters here (ASCII): A–Z, plus the only two non-ASCII code
 * points whose lower case is ASCII, U+0130 → `i` and U+212A (Kelvin) → `k`.
 * `String.prototype.toLowerCase` differs (U+0130 becomes two code points).
 */
const foldCp = (cp: number): number =>
  cp >= 0x41 && cp <= 0x5a ? cp + 0x20 : cp === 0x130 ? 0x69 : cp === 0x212a ? 0x6b : cp;

/** Case-folds a string with {@link foldCp}; used for the blocklist lookup. */
export const foldCase = (s: string): string =>
  String.fromCodePoint(...codePoints(s).map(foldCp));

const violatesLength = (cps: number[]) => cps.length < MIN_LEN || cps.length > MAX_LEN;

const violatesCharset = (cps: number[]) => cps.some((cp) => cp < 0x20 || cp > 0x7e);

/** Upper, lower, digit, symbol (printable non-alphanumeric ASCII, space included). Non-ASCII is no class. */
function violatesComplexity(cps: number[]): boolean {
  let upper = false, lower = false, digit = false, symbol = false;
  for (const cp of cps) {
    if (cp >= 0x41 && cp <= 0x5a) upper = true;
    else if (cp >= 0x61 && cp <= 0x7a) lower = true;
    else if (cp >= 0x30 && cp <= 0x39) digit = true;
    else if (cp >= 0x20 && cp <= 0x7e) symbol = true;
  }
  return [upper, lower, digit, symbol].filter(Boolean).length < MIN_CLASSES;
}

/** The four rows of US QWERTY; rows do not join and do not wrap. */
const KEYBOARD_ROWS = ["1234567890-=", "qwertyuiop[]", "asdfghjkl;'", "zxcvbnm,./"];
/** The shifted glyph of each key, position for position (letters are folded by case instead). */
const SHIFTED_ROWS = ["!@#$%^&*()_+", "QWERTYUIOP{}", "ASDFGHJKL:\"", "ZXCVBNM<>?"];

/** Glyph → [row, column]. Glyphs on no row (` ~ \ | and anything non-ASCII) break a run. */
const KEY_OF = new Map<number, [number, number]>();
[KEYBOARD_ROWS, SHIFTED_ROWS].forEach((rows) =>
  rows.forEach((row, r) =>
    Array.from(row).forEach((g, c) => KEY_OF.set(g.codePointAt(0)!, [r, c])),
  ),
);

/**
 * Longest run of positions that step by exactly ±1 in one direction within one
 * group reaches `min`. `pos` maps a code point to [group, position] or undefined
 * (which breaks the run). Same position twice is not a step (that is REPEAT).
 * Shared by KEYBOARD_RUN (group = row) and SEQUENCE (group = alphabet).
 */
function hasRun(
  cps: number[],
  pos: (cp: number) => readonly [number, number] | undefined,
  min: number,
): boolean {
  let prev: readonly [number, number] | undefined;
  let run = 0, dir = 0;
  for (const cp of cps) {
    const k = pos(cp);
    if (!k) {
      prev = undefined;
      run = 0;
      dir = 0;
      continue;
    }
    const step = prev && k[0] === prev[0] ? k[1] - prev[1] : 0;
    if (step === 1 || step === -1) {
      if (run >= 2 && step === dir) run++;
      else {
        run = 2;
        dir = step;
      }
    } else {
      run = 1;
      dir = 0;
    }
    prev = k;
    if (run >= min) return true;
  }
  return false;
}

/** 4+ adjacent keys on one row, either direction, case-insensitive, shifted glyph = its key. */
const violatesKeyboardRun = (cps: number[]) =>
  hasRun(cps.map(foldCp), (cp) => KEY_OF.get(cp), MIN_KEYBOARD_RUN);

/** Letters a–z (case folded) are group 0, digits 0–9 group 1; anything else breaks a sequence. */
function seqKey(cp: number): [number, number] | undefined {
  if (cp >= 0x61 && cp <= 0x7a) return [0, cp - 0x61];
  if (cp >= 0x41 && cp <= 0x5a) return [0, cp - 0x41];
  if (cp >= 0x30 && cp <= 0x39) return [1, cp - 0x30];
  return undefined;
}

/** 4+ consecutive letters or digits, ascending or descending; no wrap; letters and digits never join. */
const violatesSequence = (cps: number[]) => hasRun(cps, seqKey, MIN_SEQUENCE);

/** The same code point 3+ times in a row, compared exactly ("aAa" is fine). */
function violatesRepeat(cps: number[]): boolean {
  let run = 0, prev = -1;
  for (const cp of cps) {
    if (cp === prev) run++;
    else {
      run = 1;
      prev = cp;
    }
    if (run > MAX_REPEAT) return true;
  }
  return false;
}

/**
 * Every violated rule in table order. `empNo` is "" when the employee ID is
 * malformed (CONTAINS_EMP_NO is then skipped, as on the server). Without
 * `isCommon` (blocklist not loaded) COMMON_PASSWORD is never reported.
 */
export function checkPassword(
  password: string,
  empNo: string,
  isCommon?: CommonCheck,
): PasswordRule[] {
  const cps = codePoints(password);
  const violated: Record<PasswordRule, boolean> = {
    LENGTH: violatesLength(cps),
    CHARSET: violatesCharset(cps),
    COMPLEXITY: violatesComplexity(cps),
    COMMON_PASSWORD: isCommon?.(password) ?? false,
    KEYBOARD_RUN: violatesKeyboardRun(cps),
    SEQUENCE: violatesSequence(cps),
    REPEAT: violatesRepeat(cps),
    CONTAINS_EMP_NO: empNo !== "" && password.includes(empNo),
  };
  return PASSWORD_RULES.filter((r) => violated[r]);
}

/** The employee ID as the server sees it for CONTAINS_EMP_NO: "" unless it is well-formed. */
export const empNoForPolicy = (empNo: string): string => (/^[0-9]{6}$/.test(empNo) ? empNo : "");
