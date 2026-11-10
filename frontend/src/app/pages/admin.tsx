import { useTranslation } from 'react-i18next';
import { DeadlineForm } from '@/features/deadline/DeadlineForm';
import { DeadlineStates } from '@/features/deadline/DeadlineStates';
import heroLandscape from '@/shared/assets/hero-landscape.webp';
import { PageHero } from '@/shared/ui/page-hero';
import { PageHeading } from '../layouts';

/** Full-bleed hero inside the shell's padded `<main>` (shared brief: AppShell stays unchanged). */
const HERO_BLEED = '-mx-4 -mt-4 sm:-mx-8 sm:-mt-8 xl:-mx-14';

/** Admin-only pages are lazy-loaded so ordinary Users never download them (Frontend.md §7). */
export function AdminDeadline() {
  const { t } = useTranslation();
  // No page-level banner: the shell's DeadlinePill (UI-01) already shows the state on every page.
  return (
    <>
      <PageHero
        className={HERO_BLEED}
        eyebrow={t('pages.adminDeadline.eyebrow')}
        title={<PageHeading title={t('nav.deadline')} />}
        subtitle={t('pages.adminDeadline.description')}
        image={heroLandscape}
      />
      <div className="grid items-start gap-10 sm:pt-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,400px)] xl:gap-14">
        <DeadlineForm />
        <DeadlineStates />
      </div>
    </>
  );
}

export function AdminStats() {
  const { t } = useTranslation();
  return (
    <>
      <PageHero className={HERO_BLEED} eyebrow={t('pages.adminStats.eyebrow')} title={<PageHeading title={t('nav.stats')} />} image={heroLandscape} />
      <p className="sm:pt-4">{t('pages.adminStats.empty')}</p>
    </>
  );
}
