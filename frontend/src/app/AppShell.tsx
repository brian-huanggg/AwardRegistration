import { useRef, useState, type ReactNode } from 'react';
import { ChevronRight, Languages, Menu, X } from 'lucide-react';
import { Dialog } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { DeadlinePill } from '@/features/deadline/DeadlinePill';
import { setLocale } from '@/shared/i18n';
import { Button } from '@/shared/ui/button';
import { crumbsFor, type Crumb } from './breadcrumb';
import type { RouteDef } from './routes';
import type { Me } from './session';
import { Sidebar } from './Sidebar';

/**
 * The one layout for every signed-in page (Frontend.md §6.5, UI-01): skip link, sidebar (a drawer
 * below 1024 px), top bar, and the page's single `<main id="main">`. `route` is the allowed route,
 * or undefined for the 404 page; the breadcrumb is derived from it, so no page passes one in.
 */
export function AppShell({ me, route, children }: { me: Me; route?: RouteDef; children: ReactNode }) {
  const { t } = useTranslation();
  const mainRef = useRef<HTMLElement>(null);
  return (
    <div className="flex min-h-screen">
      <a
        href="#main"
        onClick={(e) => {
          e.preventDefault();
          mainRef.current?.focus();
        }}
        className="sr-only z-50 rounded-md bg-background px-3 py-2 text-sm font-medium focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:ring-2 focus:ring-ring"
      >
        {t('nav.skip')}
      </a>
      <aside className="sticky top-0 hidden h-screen w-64 shrink-0 border-r border-sidebar-border lg:block">
        <Sidebar me={me} />
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar me={me} route={route} />
        <main ref={mainRef} id="main" tabIndex={-1} className="flex-1 space-y-6 p-4 outline-none sm:p-8">
          {children}
        </main>
      </div>
    </div>
  );
}

/** Breadcrumb on the left; menu button (below 1024 px), submission state and language switch. */
function TopBar({ me, route }: { me: Me; route?: RouteDef }) {
  return (
    <header className="flex min-h-14 flex-wrap items-center gap-x-3 gap-y-2 border-b px-4 py-2 sm:px-8">
      <SidebarDrawer me={me} />
      <Breadcrumb crumbs={crumbsFor(route, me.role)} />
      <div className="ml-auto flex min-w-0 items-center gap-3">
        {/* /admin/deadline still renders its own DeadlineBanner (same region label) until UI-07 replaces it. */}
        {route?.path !== '/admin/deadline' && <DeadlinePill />}
        <LanguageSwitch />
      </div>
    </header>
  );
}

/** Below 1024 px the sidebar opens as a modal drawer: focus moves in, is trapped, Esc closes and returns it. */
function SidebarDrawer({ me }: { me: Me }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger asChild>
        <Button type="button" variant="ghost" size="icon-sm" aria-label={t('nav.openMenu')} className="lg:hidden">
          <Menu aria-hidden="true" />
        </Button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40 lg:hidden" />
        <Dialog.Content aria-describedby={undefined} className="fixed inset-y-0 left-0 z-50 w-64 max-w-[85vw] border-r border-sidebar-border shadow-lg outline-none lg:hidden">
          <Dialog.Title className="sr-only">{t('nav.main')}</Dialog.Title>
          <Sidebar me={me} onNavigate={() => setOpen(false)} />
          <Dialog.Close asChild>
            <Button type="button" variant="ghost" size="icon-sm" aria-label={t('nav.closeMenu')} className="absolute top-2 right-2">
              <X aria-hidden="true" />
            </Button>
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** `<nav>` + `<ol>`; the last crumb is the current page and not a link. Below 640 px only the current crumb shows. */
export function Breadcrumb({ crumbs }: { crumbs: Crumb[] }) {
  const { t } = useTranslation();
  return (
    <nav aria-label={t('nav.breadcrumb')} className="min-w-0">
      <ol className="flex flex-wrap items-center gap-1.5 text-sm">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          return (
            <li key={c.key} className={last ? 'flex min-w-0 items-center gap-1.5' : 'hidden items-center gap-1.5 text-muted-foreground sm:flex'}>
              {last ? (
                <span aria-current="page" className="truncate font-medium">
                  {t(c.key)}
                </span>
              ) : c.to ? (
                <Link to={c.to} className="rounded-sm hover:text-foreground hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none">
                  {t(c.key)}
                </Link>
              ) : (
                <span>{t(c.key)}</span>
              )}
              {!last && <ChevronRight aria-hidden="true" className="size-3.5 shrink-0" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

/** Toggles zh-TW ⇄ en; `setLocale` updates `<html lang>` and remembers the choice. */
export function LanguageSwitch() {
  const { t, i18n } = useTranslation();
  const next = i18n.language === 'zh-TW' ? 'en' : 'zh-TW';
  return (
    <Button type="button" variant="ghost" size="sm" aria-label={t('nav.switchLanguage')} onClick={() => void setLocale(next)}>
      <Languages aria-hidden="true" />
      <span>
        <span lang="en">EN</span> / <span lang="zh-TW">中文</span>
      </span>
    </Button>
  );
}
