// Slide rules of PRD §6.8 (Frontend.md §5.5). The back end implements the same
// rules (backend/internal/storage); both run testdata/expected-filename.json.
// The server is the final judge: this is a pre-check that saves a doomed upload.

export const CATEGORIES = ['COOL_IDEA', 'OUTSTANDING_IMPACT'] as const;
export type Category = (typeof CATEGORIES)[number];

/** 5 MiB, inclusive (PRD §6.8). */
export const MAX_SLIDES_BYTES = 5 * 1024 * 1024;
/** The longest expected filename in UTF-8 bytes (PRD §6.2), measured with `.pptx`. */
export const MAX_FILENAME_BYTES = 255;
const LONGEST_EXT = '.pptx';

export const SLIDE_EXTS = ['pdf', 'pptx'] as const;
export type SlideExt = (typeof SLIDE_EXTS)[number];

/**
 * `{I|O}_{org}_{leaderName}_{leaderEmpNo}_{ideaName}` without extension, by plain
 * concatenation, NFC-normalized. Inputs are the cleaned values.
 */
export function expectedFilenameBase(category: Category, org: string, leaderName: string, leaderEmpNo: string, ideaName: string): string {
  const prefix = category === 'COOL_IDEA' ? 'I' : 'O';
  return `${prefix}_${org}_${leaderName}_${leaderEmpNo}_${ideaName}`.normalize('NFC');
}

export function utf8Bytes(s: string): number {
  return new TextEncoder().encode(s).length;
}

/** True when base + `.pptx` exceeds 255 UTF-8 bytes → `FILENAME_TOO_LONG` (PRD §6.6). */
export function filenameTooLong(base: string): boolean {
  return utf8Bytes(base + LONGEST_EXT) > MAX_FILENAME_BYTES;
}

export type SlidesProblem = '' | 'REQUIRED' | 'FILE_TOO_LARGE' | 'EMPTY_FILE' | 'UNSUPPORTED_FILE_TYPE' | 'FILENAME_MISMATCH';

/** The part after the last dot, lower-cased; '' when there is none. */
function extOf(name: string): string {
  const dot = name.lastIndexOf('.');
  return dot < 0 ? '' : name.slice(dot + 1).toLowerCase();
}

const PDF_MAGIC = [0x25, 0x50, 0x44, 0x46, 0x2d]; // %PDF-
const ZIP_MAGIC = [0x50, 0x4b, 0x03, 0x04]; // PK\3\4

const startsWith = (b: Uint8Array, magic: number[]) => magic.every((m, i) => b[i] === m);

async function head(file: Blob, n: number): Promise<Uint8Array> {
  return new Uint8Array(await file.slice(0, n).arrayBuffer());
}

/**
 * Pre-check in the server's order (PRD §6.8): count, size, not empty, magic bytes
 * with an agreeing extension, filename. Stops at the first failing step.
 * `expectedBase` is null while the text fields cannot produce one; the filename
 * step is then skipped (the server does not validate the file when text is invalid).
 *
 * The client only checks the ZIP signature for a PPTX; the central directory
 * check stays with the server.
 */
export async function precheckSlides(files: readonly File[], expectedBase: string | null): Promise<SlidesProblem> {
  if (files.length !== 1) return 'REQUIRED';
  const file = files[0]!;
  if (file.size > MAX_SLIDES_BYTES) return 'FILE_TOO_LARGE';
  if (file.size === 0) return 'EMPTY_FILE';
  const name = file.name.normalize('NFC');
  const ext = extOf(name);
  const bytes = await head(file, PDF_MAGIC.length);
  const typeOk = (ext === 'pdf' && startsWith(bytes, PDF_MAGIC)) || (ext === 'pptx' && startsWith(bytes, ZIP_MAGIC));
  if (!typeOk) return 'UNSUPPORTED_FILE_TYPE';
  if (expectedBase === null) return '';
  const base = name.slice(0, name.length - ext.length - 1);
  return base === expectedBase.normalize('NFC') ? '' : 'FILENAME_MISMATCH';
}
