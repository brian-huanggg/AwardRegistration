import { useQuery } from '@tanstack/react-query';
import { parseApiError, type components } from '@/shared/api';

export type Deadline = components['schemas']['Deadline'];
export type SubmissionState = Deadline['state'];

export const deadlineKey = ['deadline'] as const;

export async function fetchDeadline(signal?: AbortSignal): Promise<Deadline> {
  const res = await fetch(`${location.origin}/api/v1/deadline`, { headers: { Accept: 'application/json' }, signal });
  if (!res.ok) throw await parseApiError(res);
  return (await res.json()) as Deadline;
}

/** Public endpoint, so it works for visitors too. Refetches on window focus (TanStack default). */
export function useDeadline() {
  return useQuery({ queryKey: deadlineKey, queryFn: ({ signal }) => fetchDeadline(signal), retry: false });
}

