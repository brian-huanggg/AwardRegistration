import { useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { CalendarClock, ChartColumn, CirclePlus, Lightbulb, LogOut, UserPlus, type LucideIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { NavLink, matchPath, useLocation, useNavigate } from 'react-router';
import { logout } from '@/features/auth/login';
import { Button } from '@/shared/ui/button';
import { cn } from '@/shared/ui/utils';
import { ideasKey, sectionKey } from './breadcrumb';
import { navFor, type RouteDef } from './routes';
import { replaceIdentity, type Me } from './session';

type NavKey = NonNullable<RouteDef['navKey']>;

/** Lucide icon per navigation item, named as in the Pencil Sidebar (`O:PV1ln`). Kept here so routes.ts has no React imports. */
export const NAV_ICON: Record<NavKey, LucideIcon> = {
  ideas: Lightbulb,
  ideaNew: CirclePlus,
  joinRequests: UserPlus,
  deadline: CalendarClock,
  stats: ChartColumn,
};

/** Visible label of a navigation item; the Idea list reads "My Ideas" or "All Ideas" by role. */
export function navLabelKey(key: NavKey, role: Me['role']): string {
  return key === 'ideas' ? ideasKey(role) : `nav.${key}`;
}

/** The detail and edit pages belong to the Idea list: highlighted with it, but not `aria-current="page"`. */
const IDEA_SUBPAGES = ['/ideas/:id', '/ideas/:id/edit'];

/**
 * Sidebar (Frontend.md §6.5): app name, the navigation derived from the route table for the role
 * (`navFor`), and the user menu. `onNavigate` lets the drawer close when a link is followed.
 */
export function Sidebar({ me, onNavigate, className }: { me: Me; onNavigate?: () => void; className?: string }) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const inIdea = pathname !== '/ideas/new' && IDEA_SUBPAGES.some((p) => matchPath(p, pathname));
  return (
    <div className={cn('flex h-full flex-col gap-4 bg-sidebar p-2 text-sidebar-foreground', className)}>
      <div className="flex items-center gap-2 p-2">
        <img src="/favicon.png" alt="" aria-hidden="true" className="size-8 shrink-0" />
        <span className="truncate text-sm font-medium">{t('app.name')}</span>
      </div>
      <nav aria-label={t('nav.main')} className="flex-1 overflow-y-auto">
        <p className="px-2 pb-1 text-xs font-medium text-muted-foreground">{t(sectionKey(me.role))}</p>
        <ul className="flex flex-col gap-1">
          {navFor(me.role).map((r) => {
            const key = r.navKey!;
            const Icon = NAV_ICON[key];
            return (
              <li key={r.path}>
                <NavLink
                  to={r.path}
                  end
                  onClick={onNavigate}
                  className={({ isActive }) =>
                    cn(
                      'flex min-h-8 items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-ring',
                      (isActive || (key === 'ideas' && inIdea)) && 'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
                    )
                  }
                >
                  <Icon aria-hidden="true" className="size-4 shrink-0" />
                  <span className="truncate">{t(navLabelKey(key, me.role))}</span>
                </NavLink>
              </li>
            );
          })}
        </ul>
      </nav>
      <UserMenu me={me} />
    </div>
  );
}

/**
 * Sidebar footer: who is signed in and the sign-out button. Sign-out clears the identity locally
 * even when `POST /auth/logout` fails (the server session then expires on its own); `replaceIdentity`
 * marks the exit as "signedOut", so the route gate sends plain `/login` without `next`.
 */
export function UserMenu({ me }: { me: Me }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const running = useRef(false);

  async function signOut() {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    try {
      await logout();
    } catch {
      // network or server error: still sign out locally
    }
    replaceIdentity(client, null);
    void navigate('/login', { replace: true });
  }

  const initial = Array.from(me.name.trim())[0] ?? '?';
  return (
    <div className="flex items-center gap-2 border-t border-sidebar-border p-2 pt-3">
      <span aria-hidden="true" className="flex size-8 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-sm font-medium text-sidebar-accent-foreground">
        {initial}
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{me.name}</p>
        <p className="flex flex-wrap items-center gap-x-1 text-xs text-muted-foreground">
          <span>{me.empNo}</span>
          {me.role === 'ADMIN' && (
            <>
              <span aria-hidden="true">·</span>
              <span className="rounded-full border px-1.5 font-medium text-foreground">{t('nav.administrator')}</span>
            </>
          )}
        </p>
      </div>
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={t('nav.signOut')}
        aria-busy={busy || undefined}
        disabled={busy}
        onClick={() => void signOut()}
      >
        <LogOut aria-hidden="true" />
      </Button>
    </div>
  );
}
