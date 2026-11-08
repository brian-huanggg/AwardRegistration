import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { PageHeading } from '../layouts';

/** The one 404 page: unknown route, role mismatch and not-visible resource look identical (Frontend.md §6.3). */
export function NotFound() {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t('notFound.title')} />
      <p className="mb-4">{t('notFound.body')}</p>
      <Link to="/" className="font-medium text-primary underline underline-offset-4">
        {t('notFound.home')}
      </Link>
    </>
  );
}
