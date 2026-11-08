import { useQuery } from '@tanstack/react-query';
import { parseApiError } from '@/shared/api';
import type { Role } from './routes';

/** GET /auth/me body (docs/API/response-bodies.md); `email` is null for Admins. */
export interface Me {
  empNo: string;
  name: string;
  email: string | null;
  role: Role;
}

export const meKey = ['me'] as const;

/** A 401 means visitor → null (Frontend.md §6.3 rule 1). */
export async function fetchMe(signal?: AbortSignal): Promise<Me | null> {
  const res = await fetch(`${location.origin}/api/v1/auth/me`, { headers: { Accept: 'application/json' }, signal });
  if (res.status === 401) return null;
  if (!res.ok) throw await parseApiError(res);
  return (await res.json()) as Me;
}

export function useMe() {
  return useQuery({ queryKey: meKey, queryFn: ({ signal }) => fetchMe(signal), retry: false, staleTime: 60_000 });
}

