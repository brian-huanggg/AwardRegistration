import type { RouteDef, Role } from './routes';

/** One breadcrumb entry: an i18n key, and a link target for every crumb but the current one. */
export interface Crumb {
  key: string;
  to?: string;
}

/** Sidebar section and breadcrumb root for a role (UI-01). */
export const sectionKey = (role: Role) => (role === 'ADMIN' ? 'nav.sectionAdmin' : 'nav.sectionContest');

/** "My Ideas" for Users, "All Ideas" for Admins (PRD §6.6: Users see only their own Ideas). */
export const ideasKey = (role: Role) => (role === 'ADMIN' ? 'nav.allIdeas' : 'nav.myIdeas');

/**
 * Breadcrumb for the matched route; `route` undefined = the 404 page (also for a role mismatch, so
 * the crumbs reveal nothing). The section crumb is not a link: there is no section page. The Idea
 * name is never shown: fetching it here would write an IDEA_VIEW audit row for Admins.
 */
export function crumbsFor(route: RouteDef | undefined, role: Role): Crumb[] {
  const section: Crumb = { key: sectionKey(role) };
  const ideas: Crumb = { key: ideasKey(role), to: '/ideas' };
  if (!route) return [section, { key: 'notFound.title' }];
  switch (route.path) {
    case '/ideas':
      return [section, { key: ideas.key }];
    case '/ideas/new':
      return [section, ideas, { key: 'pages.ideaCreate.title' }];
    case '/ideas/:id':
      return [section, ideas, { key: 'pages.ideaDetail.title' }];
    case '/ideas/:id/edit':
      return [section, ideas, { key: 'pages.ideaEdit.title' }];
    default:
      return [section, { key: `nav.${route.titleKey}` }];
  }
}
