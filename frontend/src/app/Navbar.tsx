import { useEffect, useId, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ChevronDown, ExternalLink, LogOut, Menu, Sprout, X } from 'lucide-react';
import { Dialog } from 'radix-ui';
import { useTranslation } from 'react-i18next';
import { Link, NavLink, useLocation, useNavigate } from 'react-router';
import { logout } from '@/features/auth/login';
import { DeadlinePill } from '@/features/deadline/DeadlinePill';
import { setLocale } from '@/shared/i18n';
import { Button } from '@/shared/ui/button';
import { cn } from '@/shared/ui/utils';
import { guideUrl, inIdeaSubpage, navLabelKey } from './nav';
import { navFor } from './routes';
import { replaceIdentity, type Me } from './session';

/**
 * Eco top navbar (Pencil Navbar/User X039Cq, Navbar/Admin gKZNx): brand, the navigation derived
 * from the route table for the role (`navFor`) plus the optional external Guide link, and on the
 * right the submission-state pill, the language switch and the user menu. Below 1024 px the
 * navigation moves into a modal menu; below 768 px the pill wraps to its own row.
 */
export function Navbar({ me }: { me: Me }) {
  return (
    <header className="border-b bg-background">
      <div className="flex min-h-[84px] flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 sm:px-8 lg:gap-x-6 xl:px-14">
        <MobileNav me={me} />
        <Brand />
        <NavItems me={me} className="hidden lg:flex lg:flex-1 lg:justify-center" listClassName="flex-row items-center gap-8 xl:gap-10" />
        <div className="order-last flex basis-full md:order-none md:ml-auto md:basis-auto lg:ml-0">
          <DeadlinePill />
        </div>
        <div className="ml-auto flex items-center gap-2 md:ml-0 xl:gap-4">
          <LanguageSwitch />
          <UserMenu me={me} />
        </div>
      </div>
    </header>
  );
}

function Brand() {
  const { t } = useTranslation();
  return (
    <Link to="/ideas" className="flex shrink-0 items-center gap-3 rounded-full outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
      <span aria-hidden="true" className="flex size-[34px] items-center justify-center rounded-full bg-brand-forest">
        <Sprout className="size-[18px] text-accent-lime" />
      </span>
      <span className="hidden text-lg font-semibold tracking-[0.09em] whitespace-nowrap text-brand-ink uppercase sm:inline">{t('app.name')}</span>
      <span className="sr-only sm:hidden">{t('app.name')}</span>
    </Link>
  );
}

/** The `<nav>` landmark with one link per route item, active item underlined, plus the external Guide link. */
function NavItems({ me, onNavigate, className, listClassName }: { me: Me; onNavigate?: () => void; className?: string; listClassName?: string }) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const inIdea = inIdeaSubpage(pathname);
  const guide = guideUrl(me.role);
  const item =
    'inline-flex min-h-9 items-center gap-1.5 border-b-2 pt-2 pb-1.5 text-sm font-medium tracking-[0.1em] whitespace-nowrap text-brand-ink uppercase outline-none hover:border-border focus-visible:ring-[3px] focus-visible:ring-ring/50';
  return (
    <nav aria-label={t('nav.main')} className={className}>
      <ul className={cn('flex', listClassName)}>
        {navFor(me.role).map((r) => {
          const key = r.navKey!;
          return (
            <li key={r.path}>
              <NavLink
                to={r.path}
                end
                onClick={onNavigate}
                className={({ isActive }) => cn(item, isActive || (key === 'ideas' && inIdea) ? 'border-brand-ink' : 'border-transparent')}
              >
                {t(navLabelKey(key, me.role))}
              </NavLink>
            </li>
          );
        })}
        {guide && (
          <li>
            <a href={guide} target="_blank" rel="noopener noreferrer" onClick={onNavigate} className={cn(item, 'border-transparent')}>
              {t('nav.guide')}
              <ExternalLink aria-hidden="true" className="size-3.5" />
              <span className="sr-only"> {t('nav.opensInNewTab')}</span>
            </a>
          </li>
        )}
      </ul>
    </nav>
  );
}

/** Below 1024 px the navigation opens as a modal menu: focus moves in, is trapped, Esc closes and returns it. */
function MobileNav({ me }: { me: Me }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger asChild>
        <Button type="button" variant="ghost" size="icon-sm" aria-label={t('nav.openMenu')} className="rounded-full lg:hidden">
          <Menu aria-hidden="true" />
        </Button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-brand-ink/40 lg:hidden" />
        <Dialog.Content aria-describedby={undefined} className="fixed inset-y-0 left-0 z-50 w-72 max-w-[85vw] overflow-y-auto border-r bg-background p-6 shadow-lg outline-none lg:hidden">
          <Dialog.Title className="mb-6 pr-10 text-sm font-semibold tracking-[0.09em] text-brand-ink uppercase">{t('nav.main')}</Dialog.Title>
          <NavItems me={me} onNavigate={() => setOpen(false)} listClassName="flex-col items-start gap-3" />
          <Dialog.Close asChild>
            <Button type="button" variant="ghost" size="icon-sm" aria-label={t('nav.closeMenu')} className="absolute top-4 right-4 rounded-full">
              <X aria-hidden="true" />
            </Button>
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** Toggles zh-TW ⇄ en; `setLocale` updates `<html lang>` and remembers the choice. */
export function LanguageSwitch() {
  const { t, i18n } = useTranslation();
  const next = i18n.language === 'zh-TW' ? 'en' : 'zh-TW';
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      aria-label={t('nav.switchLanguage')}
      onClick={() => void setLocale(next)}
      className="rounded-full text-[13px] tracking-[0.06em] text-brand-ink"
    >
      <span>
        <span lang="en">EN</span> / <span lang="zh-TW">中文</span>
      </span>
    </Button>
  );
}

/**
 * Account disclosure (Pencil "Account"): avatar initial, name, employee ID · role; opens a panel
 * with Sign out. Esc and a click outside close it. Sign-out clears the identity locally even when
 * `POST /auth/logout` fails (the server session then expires on its own); `replaceIdentity` marks
 * the exit as "signedOut", so the route gate sends plain `/login` without `next`. The email is never shown.
 */
export function UserMenu({ me }: { me: Me }) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const running = useRef(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const panelId = useId();

  useEffect(() => {
    if (!open) return;
    const onPointer = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      setOpen(false);
      trigger.current?.focus();
    };
    document.addEventListener('pointerdown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

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
    <div
      ref={root}
      className="relative"
      onBlur={(e) => {
        if (open && e.relatedTarget && !root.current?.contains(e.relatedTarget as Node)) setOpen(false);
      }}
    >
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2.5 rounded-full border bg-background p-1.5 outline-none hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring/50 md:pr-4"
      >
        {/* Avatar: --brand-purple (white text 5.9:1); the design's #9786B5 gives 3.3:1. */}
        <span aria-hidden="true" className="flex size-8 shrink-0 items-center justify-center rounded-full bg-brand-purple text-sm font-semibold text-white">
          {initial}
        </span>
        <span className="sr-only text-left md:not-sr-only md:flex md:flex-col">
          <span className="max-w-40 truncate text-[13px] font-semibold text-brand-ink">{me.name}</span>
          <span className="flex items-center gap-1 text-[11px] tracking-[0.05em] text-muted-foreground uppercase">
            <span>{me.empNo}</span>
            <span aria-hidden="true">·</span>
            <span>{t(me.role === 'ADMIN' ? 'nav.administrator' : 'nav.roleUser')}</span>
          </span>
        </span>
        <ChevronDown aria-hidden="true" className={cn('hidden size-4 text-muted-foreground transition-transform md:block', open && 'rotate-180')} />
      </button>
      <div id={panelId} hidden={!open} className="absolute top-full right-0 z-30 mt-2 w-56 rounded-2xl border bg-popover p-2 text-popover-foreground shadow-lg">
        <Button
          type="button"
          variant="ghost"
          aria-busy={busy || undefined}
          disabled={busy}
          onClick={() => void signOut()}
          className="w-full justify-start rounded-xl"
        >
          <LogOut aria-hidden="true" />
          {t('nav.signOut')}
        </Button>
      </div>
    </div>
  );
}
