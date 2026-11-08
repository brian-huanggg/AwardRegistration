import { useEffect, useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { NavLink, Link } from 'react-router';
import { DeadlineBanner } from '@/features/deadline/DeadlineBanner';
import { LOCALES, setLocale, type Locale } from '@/shared/i18n';
import { navFor } from './routes';
import type { Me } from './session';

function LanguageSwitch() {
  const { t, i18n } = useTranslation();
  return (
    <label className="flex items-center gap-2 text-sm">
      <span>{t('common.language')}</span>
      <select
        className="rounded-md border bg-background px-2 py-1"
        value={i18n.language}
        onChange={(e) => void setLocale(e.target.value as Locale)}
      >
        {LOCALES.map((l) => (
          <option key={l} value={l}>
            {l === 'zh-TW' ? '繁體中文' : 'English'}
          </option>
        ))}
      </select>
    </label>
  );
}

/** One h1 per page; after a route change focus moves to it and the title updates (Frontend.md §8). */
export function PageHeading({ title }: { title: string }) {
  const { t } = useTranslation();
  const ref = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    document.title = `${title} · ${t('app.name')}`;
    ref.current?.focus();
  }, [title, t]);
  return (
    <h1 ref={ref} tabIndex={-1} className="mb-4 text-2xl font-semibold outline-none">
      {title}
    </h1>
  );
}

function Frame({ header, children }: { header: ReactNode; children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <div className="min-h-screen">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:z-10 focus:bg-background focus:p-2">
        {t('nav.skip')}
      </a>
      <header className="border-b">{header}</header>
      <DeadlineBanner />
      <main id="main" className="mx-auto max-w-5xl p-4">
        {children}
      </main>
    </div>
  );
}

export function PublicLayout({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <Frame
      header={
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 p-4">
          <Link to="/login" className="font-semibold">
            {t('app.name')}
          </Link>
          <LanguageSwitch />
        </div>
      }
    >
      {children}
    </Frame>
  );
}

/** Logged-in layout; navigation comes from the route table and the role. */
export function AppShell({ me, children }: { me: Me; children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <Frame
      header={
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-4 p-4">
          <nav aria-label={t('nav.main')}>
            <ul className="flex flex-wrap items-center gap-4">
              {navFor(me.role).map((r) => (
                <li key={r.path}>
                  <NavLink to={r.path} className="underline-offset-4 hover:underline aria-[current=page]:font-semibold">
                    {t(`nav.${r.navKey!}`)}
                  </NavLink>
                </li>
              ))}
            </ul>
          </nav>
          <div className="flex items-center gap-4">
            {me.role === 'ADMIN' && <span className="rounded-md border px-2 py-0.5 text-sm">{t('nav.administrator')}</span>}
            <span className="text-sm">{me.name}</span>
            <LanguageSwitch />
          </div>
        </div>
      }
    >
      {children}
    </Frame>
  );
}
