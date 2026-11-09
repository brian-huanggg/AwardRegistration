import { useTranslation } from 'react-i18next';
import { DeadlineForm } from '@/features/deadline/DeadlineForm';
import { DeadlineStates } from '@/features/deadline/DeadlineStates';
import { PageHeading } from '../layouts';

/** Admin-only pages are lazy-loaded so ordinary Users never download them (Frontend.md §7). */
export function AdminDeadline() {
  const { t } = useTranslation();
  // No page-level banner: the shell's DeadlinePill (UI-01) already shows the state on every page.
  return (
    <>
      <div>
        <PageHeading title={t('nav.deadline')} />
        <p className="mt-1 max-w-3xl text-sm text-muted-foreground">{t('pages.adminDeadline.description')}</p>
      </div>
      <div className="grid items-start gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <DeadlineForm />
        </div>
        <DeadlineStates />
      </div>
    </>
  );
}

export function AdminStats() {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t('nav.stats')} />
      <p>{t('pages.adminStats.empty')}</p>
    </>
  );
}
