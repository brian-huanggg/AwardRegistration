import { useTranslation } from 'react-i18next';
import { DeadlineBanner } from '@/features/deadline/DeadlineBanner';
import { DeadlineForm } from '@/features/deadline/DeadlineForm';
import { PageHeading } from '../layouts';

/** Admin-only pages are lazy-loaded so ordinary Users never download them (Frontend.md §7). */
export function AdminDeadline() {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t('nav.deadline')} />
      <div className="mb-6 max-w-md overflow-hidden rounded-xl border">
        <DeadlineBanner />
      </div>
      <DeadlineForm />
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
