import { matchPath } from 'react-router';
import type { RouteDef, Role } from './routes';

export type NavKey = NonNullable<RouteDef['navKey']>;

/** "My Ideas" for Users, "All Ideas" for Admins (PRD §6.6: Users see only their own Ideas). Used as the list's page title. */
export const ideasKey = (role: Role) => (role === 'ADMIN' ? 'nav.allIdeas' : 'nav.myIdeas');

/** Visible label of a navbar item (Eco Navbar X039Cq / gKZNx): the Admin's Idea list reads "Registrations". */
export function navLabelKey(key: NavKey, role: Role): string {
  if (key === 'ideas') return role === 'ADMIN' ? 'nav.registrations' : 'nav.myIdeas';
  return `nav.${key}`;
}

/** The detail and edit pages belong to the Idea list: highlighted with it, but not `aria-current="page"`. */
export function inIdeaSubpage(pathname: string): boolean {
  return pathname !== '/ideas/new' && ['/ideas/:id', '/ideas/:id/edit'].some((p) => matchPath(p, pathname));
}

/**
 * The "Guide" navbar item is an external link (no page exists). Its URL comes from the build-time
 * env `VITE_GUIDE_URL`; unset or blank hides the item. Users only (the Admin frame has no Guide).
 */
export function guideUrl(role: Role): string | undefined {
  if (role !== 'USER') return undefined;
  const url = import.meta.env.VITE_GUIDE_URL?.trim();
  return url ? url : undefined;
}
