import { ApiError, apiFetch, type components } from '@/shared/api';
import { checkName } from '@/shared/text';

export type JoinRequest = components['schemas']['JoinRequest'];

export const joinRequestsKey = ['join-requests'] as const;
export const IDEA_NAME_MAX = 75;

export type JoinField = 'leaderEmpNo' | 'ideaName';
/** A field problem is an `errors.<code>` key. */
export type JoinProblems = Partial<Record<JoinField, string>>;

/** Same checks as the server (PRD §6.7): six ASCII digits, a cleaned name of 1–75 code points. */
export function precheckJoin(v: { leaderEmpNo: string; ideaName: string }): { values: { leaderEmpNo: string; ideaName: string }; problems: JoinProblems } {
  const problems: JoinProblems = {};
  const leaderEmpNo = v.leaderEmpNo.trim();
  if (leaderEmpNo === '') problems.leaderEmpNo = 'REQUIRED';
  else if (!/^[0-9]{6}$/.test(leaderEmpNo)) problems.leaderEmpNo = 'INVALID_FORMAT';
  const name = checkName(v.ideaName, IDEA_NAME_MAX);
  if (name.problem) problems.ideaName = name.problem;
  return { values: { leaderEmpNo, ideaName: name.cleaned }, problems };
}

export async function fetchJoinRequests(signal?: AbortSignal): Promise<JoinRequest[]> {
  const res = await apiFetch('/join-requests', { signal });
  return ((await res.json()) as { items: JoinRequest[] }).items;
}

/** POST /join-requests: the Idea is found by the Leader's employee ID and the Idea name. */
export async function createJoinRequest(values: { leaderEmpNo: string; ideaName: string }): Promise<JoinRequest> {
  const res = await apiFetch('/join-requests', { method: 'POST', body: { ...values } });
  return (await res.json()) as JoinRequest;
}

/** DELETE /ideas/{id}/members/me: withdraws the caller's own pending request (204). */
export async function withdrawJoinRequest(ideaId: string): Promise<void> {
  await apiFetch(`/ideas/${encodeURIComponent(ideaId)}/members/me`, { method: 'DELETE' });
}

/** Field problems from a 422; `rest` is everything else and is shown as a form-level message. */
export function mapJoinError(e: unknown): { problems: JoinProblems; rest: ApiError | null } {
  if (!(e instanceof ApiError)) return { problems: {}, rest: new ApiError({ status: 0, code: 'UNKNOWN' }) };
  const problems: JoinProblems = {};
  for (const fe of e.errors) {
    if ((fe.field === 'leaderEmpNo' || fe.field === 'ideaName') && problems[fe.field] === undefined) problems[fe.field] = fe.code;
  }
  return { problems, rest: Object.keys(problems).length > 0 ? null : e };
}

/**
 * A 404 on apply means "no Idea for this Leader ID and name". The answer is uniform (AGENTS.md), so the
 * UI shows one message under the Idea name field that never says which of the two inputs was wrong.
 */
export function isJoinNotFound(e: unknown): boolean {
  return e instanceof ApiError && e.status === 404;
}

/** Local calendar-day difference in the browser time zone (0 = same day, 1 = `iso` was the day before `now`). */
function daysBefore(iso: string, now: number): number {
  const a = new Date(iso);
  const b = new Date(now);
  return Math.round((Date.UTC(b.getFullYear(), b.getMonth(), b.getDate()) - Date.UTC(a.getFullYear(), a.getMonth(), a.getDate())) / 86_400_000);
}

export type SentAt = { kind: 'today' | 'yesterday'; time: string } | { kind: 'date'; date: string };

/** How a sent time is shown: relative day plus time for today and yesterday, a medium date otherwise. */
export function sentAtParts(iso: string, now: number, locale: string): SentAt {
  const at = new Date(iso);
  const diff = daysBefore(iso, now);
  if (diff === 0 || diff === 1) return { kind: diff === 0 ? 'today' : 'yesterday', time: new Intl.DateTimeFormat(locale, { timeStyle: 'short' }).format(at) };
  return { kind: 'date', date: new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(at) };
}
