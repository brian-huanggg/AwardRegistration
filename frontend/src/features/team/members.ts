import { apiFetch, type components } from '@/shared/api';

type IdeaDetail = components['schemas']['IdeaDetail'];

const base = (ideaId: string, empNo: string) => `/ideas/${encodeURIComponent(ideaId)}/members/${encodeURIComponent(empNo)}`;

/** POST …/approve: the response is the whole Idea with the new Member. */
export async function approveMember(ideaId: string, empNo: string): Promise<IdeaDetail> {
  const res = await apiFetch(`${base(ideaId, empNo)}/approve`, { method: 'POST' });
  return (await res.json()) as IdeaDetail;
}

/** DELETE …/members/{empNo}: rejects a PENDING row or removes an ACCEPTED one (204). */
export async function removeMember(ideaId: string, empNo: string): Promise<void> {
  await apiFetch(base(ideaId, empNo), { method: 'DELETE' });
}

/** DELETE …/members/me: the caller leaves the team (204). */
export async function leaveTeam(ideaId: string): Promise<void> {
  await apiFetch(`/ideas/${encodeURIComponent(ideaId)}/members/me`, { method: 'DELETE' });
}
