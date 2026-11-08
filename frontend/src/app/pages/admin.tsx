import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/shared/ui/button';
import { Field } from '@/shared/ui/form-field';
import { PageHeading } from '../layouts';

/** Admin-only pages are lazy-loaded so ordinary Users never download them (Frontend.md §7). */
export function AdminDeadline() {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t('nav.deadline')} />
      <form onSubmit={(e: FormEvent) => e.preventDefault()} className="max-w-md">
        <Field label={t('pages.adminDeadline.label')} hint={t('pages.adminDeadline.hint')}>
          {(p) => <input {...p} name="deadline" type="datetime-local" required />}
        </Field>
        <Button type="submit">{t('common.save')}</Button>
      </form>
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
