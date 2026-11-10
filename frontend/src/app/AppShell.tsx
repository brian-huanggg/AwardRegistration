import { useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Navbar } from './Navbar';
import type { RouteDef } from './routes';
import type { Me } from './session';

export { LanguageSwitch } from './Navbar';

/**
 * The one layout for every signed-in page (Frontend.md §6.5, Eco redesign E0): skip link, the top
 * navbar, and the page's single `<main id="main">`. `route` is the allowed route, or undefined for
 * the 404 page (no navigation item is then marked current: NavLink matches only real routes).
 */
export function AppShell({ me, children }: { me: Me; route?: RouteDef; children: ReactNode }) {
  const { t } = useTranslation();
  const mainRef = useRef<HTMLElement>(null);
  return (
    <div className="flex min-h-screen flex-col">
      <a
        href="#main"
        onClick={(e) => {
          e.preventDefault();
          mainRef.current?.focus();
        }}
        className="sr-only z-50 rounded-full bg-background px-4 py-2 text-sm font-medium focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:ring-2 focus:ring-ring"
      >
        {t('nav.skip')}
      </a>
      <Navbar me={me} />
      <main ref={mainRef} id="main" tabIndex={-1} className="flex-1 space-y-6 p-4 outline-none sm:p-8 xl:px-14">
        {children}
      </main>
    </div>
  );
}
