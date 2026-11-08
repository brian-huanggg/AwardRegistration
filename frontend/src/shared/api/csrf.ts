import { parseApiError } from './problem';

/**
 * Session-bound CSRF token, kept in memory only (PRD §6.5, Frontend.md §5.2, F13).
 * The token is bound to the current session (anonymous before login), so it is
 * dropped on login, logout and any 401 and fetched again lazily.
 */
let token: string | null = null;
let inflight: Promise<string> | null = null;
/** Bumped by clearCsrfToken so a fetch started before the clear cannot repopulate the cache. */
let generation = 0;

export const API_BASE = () => `${location.origin}/api/v1`;

async function fetchToken(): Promise<string> {
  const res = await fetch(`${API_BASE()}/auth/csrf`, { headers: { Accept: 'application/json' }, credentials: 'same-origin' });
  if (!res.ok) throw await parseApiError(res);
  const body = (await res.json()) as { token?: unknown };
  if (typeof body.token !== 'string' || body.token === '') throw await parseApiError(new Response(null, { status: 500 }));
  return body.token;
}

/** Returns the cached token, fetching it once (shared by concurrent callers) when absent. */
export function getCsrfToken(): Promise<string> {
  if (token !== null) return Promise.resolve(token);
  if (inflight) return inflight;
  const started = generation;
  const p = fetchToken()
    .then((t) => {
      if (started === generation) token = t;
      return t;
    })
    .finally(() => {
      if (inflight === p) inflight = null;
    });
  inflight = p;
  return p;
}

/** Drops the token (login, logout, 401). The next write fetches a new one. */
export function clearCsrfToken(): void {
  generation += 1;
  token = null;
  inflight = null;
}

/** Drops the token and fetches a new one (the `CSRF_INVALID` retry). */
export function refreshCsrfToken(): Promise<string> {
  clearCsrfToken();
  return getCsrfToken();
}
