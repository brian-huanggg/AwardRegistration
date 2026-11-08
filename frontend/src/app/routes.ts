/** The single route table (Frontend.md §6.2). Guards, navigation and `next` validation all read it. */
export type Access = 'public' | 'USER' | 'ADMIN' | 'ANY';
export type Role = 'USER' | 'ADMIN';

export interface RouteDef {
  /** React Router path pattern. */
  path: string;
  access: Access;
  /** i18n key under `nav.*`; present only for routes shown in the navigation. */
  navKey?: 'ideas' | 'joinRequests' | 'deadline' | 'stats';
  /** i18n key under `nav.*` used as the page title / h1. */
  titleKey: 'ideas' | 'joinRequests' | 'deadline' | 'stats' | 'signIn' | 'register';
}

export const ROUTES: readonly RouteDef[] = [
  { path: '/login', access: 'public', titleKey: 'signIn' },
  { path: '/register', access: 'public', titleKey: 'register' },
  { path: '/ideas', access: 'ANY', navKey: 'ideas', titleKey: 'ideas' },
  { path: '/ideas/new', access: 'USER', titleKey: 'ideas' },
  { path: '/ideas/:id', access: 'ANY', titleKey: 'ideas' },
  { path: '/ideas/:id/edit', access: 'USER', titleKey: 'ideas' },
  { path: '/join-requests', access: 'USER', navKey: 'joinRequests', titleKey: 'joinRequests' },
  { path: '/admin/deadline', access: 'ADMIN', navKey: 'deadline', titleKey: 'deadline' },
  { path: '/admin/stats', access: 'ADMIN', navKey: 'stats', titleKey: 'stats' },
];

export function canAccess(access: Access, role: Role | null): boolean {
  if (access === 'public') return true;
  if (role === null) return false;
  return access === 'ANY' || access === role;
}

/** Navigation items for a role, in table order. */
export function navFor(role: Role): RouteDef[] {
  return ROUTES.filter((r) => r.navKey && canAccess(r.access, role));
}
