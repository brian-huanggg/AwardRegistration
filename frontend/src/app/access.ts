import { matchPath } from 'react-router';
import { ROUTES, canAccess, type Access, type Role } from './routes';

export type Decision = { kind: 'allow' } | { kind: 'login'; next: string } | { kind: 'home' } | { kind: 'notFound' };

export function findRoute(pathname: string) {
  return ROUTES.find((r) => matchPath({ path: r.path, end: true }, pathname));
}

/** Frontend.md §6.3 rules 2–4 for one route. */
export function decide(access: Access, role: Role | null, location: { pathname: string; search: string }): Decision {
  if (access === 'public') return role ? { kind: 'home' } : { kind: 'allow' };
  if (role === null) return { kind: 'login', next: location.pathname + location.search };
  return canAccess(access, role) ? { kind: 'allow' } : { kind: 'notFound' };
}

/** Frontend.md §6.4: same-origin relative path that resolves to a route the role may use, else /ideas. */
export function safeNext(next: string | null, role: Role): string {
  const fallback = '/ideas';
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.includes('\\') || /^\/*[a-z][a-z0-9+.-]*:/i.test(next)) return fallback;
  let url: URL;
  try {
    url = new URL(next, 'http://local.invalid');
  } catch {
    return fallback;
  }
  if (url.origin !== 'http://local.invalid') return fallback;
  const route = findRoute(url.pathname);
  return route && canAccess(route.access, role) && route.access !== 'public' ? url.pathname + url.search : fallback;
}
