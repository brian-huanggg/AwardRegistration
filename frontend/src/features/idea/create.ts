import { ApiError, apiFetch, xhrTransport, type components } from '@/shared/api';
import { CATEGORIES, expectedFilenameBase, filenameTooLong, precheckSlides, type Category } from '@/shared/slides';
import { checkName, checkSummary } from '@/shared/text';

export type IdeaDetail = components['schemas']['IdeaDetail'];

/** Form-field order; also the order of the error summary (error-codes.md §2). */
export const IDEA_FIELDS = ['category', 'org', 'direction', 'name', 'summary', 'slides'] as const;
export type IdeaField = (typeof IDEA_FIELDS)[number];
export type TextField = Exclude<IdeaField, 'slides'>;

export const ORGS = ['AAID', 'BSID', 'ICSD', 'TSID', 'PLED', 'DEID'] as const;
export const DIRECTIONS = [
  'GREEN_MANUFACTURING',
  'RESPONSIBLE_SUPPLY_CHAIN',
  'INCLUSIVE_WORKPLACE',
  'TALENT_DEVELOPMENT',
  'CARING_FOR_DISADVANTAGED',
] as const;
export { CATEGORIES };

export const NAME_MAX = 75;
export const SUMMARY_MAX = 300;

export type IdeaText = Record<TextField, string>;
/** A field problem is an `errors.<code>` key. */
export type FieldProblems = Partial<Record<IdeaField, string>>;

export interface Leader {
  name: string;
  empNo: string;
}

const isCategory = (v: string): v is Category => (CATEGORIES as readonly string[]).includes(v);
const isOneOf = (list: readonly string[], v: string) => list.includes(v);

/**
 * The expected filename base for the current input, or null while category, org or
 * name cannot produce one. Built from the cleaned name, as the server does.
 */
export function expectedBaseFor(v: Pick<IdeaText, 'category' | 'org' | 'name'>, leader: Leader): string | null {
  const name = checkName(v.name, NAME_MAX);
  if (!isCategory(v.category) || !isOneOf(ORGS, v.org) || name.problem !== '') return null;
  return expectedFilenameBase(v.category, v.org, leader.name, leader.empNo, name.cleaned);
}

/**
 * Text checks in the server's order (PRD §6.2, §6.6), one problem per field;
 * `FILENAME_TOO_LONG` is reported on `name` like the server does. Returns the
 * cleaned values that are sent, and the expected base when the text is valid.
 */
export function precheckText(v: IdeaText, leader: Leader): { values: IdeaText; problems: FieldProblems; expectedBase: string | null } {
  const problems: FieldProblems = {};
  const pick = (f: 'category' | 'org' | 'direction', list: readonly string[]) => {
    if (v[f] === '') problems[f] = 'REQUIRED';
    else if (!isOneOf(list, v[f])) problems[f] = 'INVALID_ENUM';
  };
  pick('category', CATEGORIES);
  pick('org', ORGS);
  pick('direction', DIRECTIONS);

  const name = checkName(v.name, NAME_MAX);
  let expectedBase: string | null = null;
  if (name.problem) problems.name = name.problem;
  else {
    expectedBase = expectedBaseFor(v, leader);
    if (expectedBase !== null && filenameTooLong(expectedBase)) {
      problems.name = 'FILENAME_TOO_LONG';
      expectedBase = null;
    }
  }

  const summary = checkSummary(v.summary, SUMMARY_MAX);
  if (summary.problem) problems.summary = summary.problem;

  return { values: { ...v, name: name.cleaned, summary: summary.cleaned }, problems, expectedBase };
}

/**
 * Whole-form pre-check. A missing file is always reported; the other file steps
 * run only when the text is valid, because the server does not validate the file
 * otherwise (PRD §6.8) and the filename depends on the text.
 */
export async function precheck(v: IdeaText, files: readonly File[], leader: Leader): Promise<{ values: IdeaText; problems: FieldProblems }> {
  const { values, problems, expectedBase } = precheckText(v, leader);
  const textOk = Object.keys(problems).length === 0;
  const slides = files.length === 0 ? 'REQUIRED' : textOk ? await precheckSlides(files, expectedBase) : '';
  if (slides) problems.slides = slides;
  return { values, problems };
}

/** POST /ideas: one multipart request with every field and the slides (PRD §6.6). */
export async function createIdea(values: IdeaText, file: File, onProgress?: (loaded: number, total: number) => void): Promise<IdeaDetail> {
  const body = new FormData();
  for (const f of ['category', 'org', 'direction', 'name', 'summary'] as const) body.append(f, values[f]);
  body.append('slides', file, file.name);
  const res = await apiFetch('/ideas', { method: 'POST', body, transport: xhrTransport(onProgress) });
  return (await res.json()) as IdeaDetail;
}

/** File errors that carry no `errors[]` entry; they belong to the slides field. */
const SLIDES_CODES = new Set(['FILE_TOO_LARGE', 'EMPTY_FILE', 'UNSUPPORTED_FILE_TYPE', 'FILENAME_MISMATCH']);

const isField = (f: string): f is IdeaField => (IDEA_FIELDS as readonly string[]).includes(f);

/**
 * Maps a failed create to field problems; `rest` has no field (403 state, 503, …)
 * and is shown as a form-level message. `expected` is the server's expected
 * filename from a `FILENAME_MISMATCH`.
 */
export function mapServerError(e: unknown): { problems: FieldProblems; rest: ApiError | null; expected?: string } {
  if (!(e instanceof ApiError)) return { problems: {}, rest: new ApiError({ status: 0, code: 'UNKNOWN' }) };
  if (e.code === 'IDEA_DUPLICATE') return { problems: { name: 'IDEA_DUPLICATE' }, rest: null };
  const problems: FieldProblems = {};
  let expected: string | undefined;
  for (const fe of e.errors) {
    if (!isField(fe.field) || problems[fe.field] !== undefined) continue;
    problems[fe.field] = fe.code;
    if (fe.field === 'slides' && typeof fe.expected === 'string') expected = fe.expected;
  }
  if (Object.keys(problems).length === 0 && SLIDES_CODES.has(e.code)) problems.slides = e.code;
  return { problems, rest: Object.keys(problems).length > 0 ? null : e, expected };
}
