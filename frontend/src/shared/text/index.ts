// Implements PRD §6.2 clean-up and counting. The back end (Go) implements the
// same rules; both run testdata/text-cleanup.json.
// String.prototype.trim is NOT used: its whitespace set differs from Unicode
// White_Space (it strips U+FEFF and keeps U+0085).

export type Problem = '' | 'REQUIRED' | 'INVALID_FORMAT' | 'INVALID_NAME_CHARS' | 'OUT_OF_RANGE';

export interface CheckResult {
  cleaned: string;
  problem: Problem;
}

/** Unicode White_Space property: 25 code points. */
export function isWhitespace(cp: number): boolean {
  return (
    (cp >= 0x0009 && cp <= 0x000d) ||
    cp === 0x0020 ||
    cp === 0x0085 ||
    cp === 0x00a0 ||
    cp === 0x1680 ||
    (cp >= 0x2000 && cp <= 0x200a) ||
    cp === 0x2028 ||
    cp === 0x2029 ||
    cp === 0x202f ||
    cp === 0x205f ||
    cp === 0x3000
  );
}

/** Characters that would end up in the expected filename (PRD §6.2 rule 8). */
const FORBIDDEN_NAME_CHARS = /[/\\:*?"<>|]/;
const CONTROL = /\p{Cc}/u;

/** Length in Unicode code points (not UTF-16 units). */
export function countCodePoints(s: string): number {
  return Array.from(s).length;
}

/** Removes leading and trailing White_Space code points. */
export function trimWhitespace(s: string): string {
  const cps = Array.from(s);
  let start = 0;
  let end = cps.length;
  while (start < end && isWhitespace(cps[start]!.codePointAt(0)!)) start++;
  while (end > start && isWhitespace(cps[end - 1]!.codePointAt(0)!)) end--;
  return cps.slice(start, end).join('');
}

/** NFC, trim, then every whitespace run becomes one ASCII space. */
export function cleanName(s: string): string {
  const trimmed = trimWhitespace(s.normalize('NFC'));
  let out = '';
  let inRun = false;
  for (const ch of trimmed) {
    if (isWhitespace(ch.codePointAt(0)!)) {
      inRun = true;
      continue;
    }
    if (inRun) {
      out += ' ';
      inRun = false;
    }
    out += ch;
  }
  return out;
}

/** NFC, CRLF and CR become LF, then trim. */
export function cleanSummary(s: string): string {
  return trimWhitespace(s.normalize('NFC').replace(/\r\n/g, '\n').replace(/\r/g, '\n'));
}

function check(cleaned: string, max: number, name: boolean): Problem {
  if (cleaned === '') return 'REQUIRED';
  for (const ch of cleaned) {
    if (CONTROL.test(ch) && (name || ch !== '\n')) return 'INVALID_FORMAT';
  }
  if (name && FORBIDDEN_NAME_CHARS.test(cleaned)) return 'INVALID_NAME_CHARS';
  if (countCodePoints(cleaned) > max) return 'OUT_OF_RANGE';
  return '';
}

/** Cleans a person / Idea name and reports its first problem; `max` is inclusive. */
export function checkName(s: string, max: number): CheckResult {
  const cleaned = cleanName(s);
  return { cleaned, problem: check(cleaned, max, true) };
}

/** Cleans a summary and reports its first problem; `max` is inclusive. */
export function checkSummary(s: string, max: number): CheckResult {
  const cleaned = cleanSummary(s);
  return { cleaned, problem: check(cleaned, max, false) };
}
