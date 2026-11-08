import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/shared/ui/button';
import { Field } from '@/shared/ui/form-field';
import { PageHeading } from '../layouts';

export function JoinRequests() {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t('nav.joinRequests')} />
      <section className="mb-8 max-w-md">
        <h2 className="mb-2 text-lg font-semibold">{t('pages.joinRequests.apply')}</h2>
        <form onSubmit={(e: FormEvent) => e.preventDefault()}>
          <Field label={t('fields.leaderEmpNo')}>{(p) => <input {...p} name="leaderEmpNo" required />}</Field>
          <Field label={t('fields.ideaName')}>{(p) => <input {...p} name="ideaName" required />}</Field>
          <Button type="submit">{t('pages.joinRequests.submit')}</Button>
        </form>
      </section>
      <section>
        <h2 className="mb-2 text-lg font-semibold">{t('pages.joinRequests.mine')}</h2>
        <p className="text-sm">{t('pages.joinRequests.empty')}</p>
      </section>
    </>
  );
}
