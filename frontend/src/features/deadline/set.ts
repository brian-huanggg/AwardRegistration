import { apiFetch } from '@/shared/api';
import type { Deadline } from './useDeadline';

/** PUT /admin/deadline. `deadline` is ISO 8601 with offset; the answer has the same body as GET /deadline. */
export async function putDeadline(deadline: string): Promise<Deadline> {
  const res = await apiFetch('/admin/deadline', { method: 'PUT', body: { deadline } });
  return (await res.json()) as Deadline;
}
