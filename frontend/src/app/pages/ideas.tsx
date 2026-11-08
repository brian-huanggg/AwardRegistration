import type { FormEvent, ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useDeadline } from '@/features/deadline/useDeadline';
import { Button } from '@/shared/ui/button';
import { Field } from '@/shared/ui/form-field';
import { PageHeading } from '../layouts';
import { useMe } from '../session';

const noSubmit = (e: FormEvent) => e.preventDefault();

function IdeaFields({ withSlides }: { withSlides: boolean }) {
  const { t } = useTranslation();
  return (
    <>
      <Field label={t('fields.ideaName')}>{(p) => <input {...p} name="name" required />}</Field>
      <Field label={t('fields.category')}>
        {(p) => (
          <select {...p} name="category" required>
            <option value="" />
          </select>
        )}
      </Field>
      <Field label={t('fields.org')}>
        {(p) => (
          <select {...p} name="org" required>
            <option value="" />
          </select>
        )}
      </Field>
      <Field label={t('fields.direction')}>
        {(p) => (
          <select {...p} name="direction" required>
            <option value="" />
          </select>
        )}
      </Field>
      <Field label={t('fields.summary')}>{(p) => <textarea {...p} name="summary" rows={6} required />}</Field>
      {withSlides && (
        <Field label={t('fields.slides')} hint={t('pages.ideaForm.slidesHint')}>
          {(p) => <input {...p} name="slides" type="file" accept=".pdf,.pptx" required />}
        </Field>
      )}
    </>
  );
}

export function IdeaList() {
  const { t } = useTranslation();
  const { data: me } = useMe();
  return (
    <>
      <PageHeading title={t('nav.ideas')} />
      <form role="search" onSubmit={noSubmit} className="mb-4 flex flex-wrap items-end gap-4">
        <Field label={t('pages.ideaList.search')}>{(p) => <input {...p} name="q" type="search" />}</Field>
        <Button type="submit" className="mb-4">
          {t('pages.ideaList.apply')}
        </Button>
        {me?.role === 'USER' && (
          <Button asChild variant="outline" className="mb-4">
            <Link to="/ideas/new">{t('pages.ideaList.create')}</Link>
          </Button>
        )}
      </form>
      <table className="w-full rounded-xl border bg-card text-left text-sm">
        <caption className="sr-only">{t('nav.ideas')}</caption>
        <thead>
          <tr>
            {(['ideaName', 'category', 'org', 'direction', 'leader', 'members'] as const).map((c) => (
              <th key={c} scope="col" className="border-b p-3 font-medium text-muted-foreground">
                {t(`fields.${c}`)}
              </th>
            ))}
          </tr>
        </thead>
      </table>
      <p className="mt-4">{t('pages.ideaList.empty')}</p>
    </>
  );
}

export function IdeaCreate() {
  const { t } = useTranslation();
  const { data: deadline } = useDeadline();
  const open = deadline?.state === 'OPEN'; // Frontend.md §6.3 rule 6: a banner and disabled controls, not a guard
  return (
    <>
      <PageHeading title={t('pages.ideaCreate.title')} />
      <form onSubmit={noSubmit} className="max-w-2xl rounded-xl border bg-card p-6">
        <IdeaFields withSlides />
        <Button type="submit" disabled={!open}>
          {t('pages.ideaCreate.submit')}
        </Button>
      </form>
    </>
  );
}

function Section({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <section className="mb-4 rounded-xl border bg-card p-6">
      <h2 className="mb-3 text-lg font-semibold">{title}</h2>
      {children}
    </section>
  );
}

export function IdeaDetail() {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t('pages.ideaDetail.title')} />
      <Section title={t('pages.ideaDetail.details')}>
        <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
          {(['ideaName', 'category', 'org', 'direction', 'leader', 'summary'] as const).map((k) => (
            <div key={k} className="contents">
              <dt className="font-medium">{t(`fields.${k}`)}</dt>
              <dd>–</dd>
            </div>
          ))}
        </dl>
      </Section>
      <Section title={t('pages.ideaDetail.members')}>
        <p className="text-sm">{t('pages.ideaDetail.noMembers')}</p>
      </Section>
      <Section title={t('pages.ideaDetail.pending')}>
        <p className="text-sm">{t('pages.ideaDetail.noPending')}</p>
      </Section>
      <Section title={t('pages.ideaDetail.slides')}>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline">{t('pages.ideaDetail.download')}</Button>
          <Button variant="outline">{t('pages.ideaDetail.replace')}</Button>
          <Button variant="destructive">{t('pages.ideaDetail.delete')}</Button>
        </div>
      </Section>
      <Link to="/ideas" className="font-medium text-primary underline underline-offset-4">
        {t('pages.ideaDetail.back')}
      </Link>
    </>
  );
}

export function IdeaEdit() {
  const { t } = useTranslation();
  return (
    <>
      <PageHeading title={t('pages.ideaEdit.title')} />
      <form onSubmit={noSubmit} className="max-w-2xl rounded-xl border bg-card p-6">
        <IdeaFields withSlides={false} />
        <div className="flex gap-2">
          <Button type="submit">{t('common.save')}</Button>
          <Button type="button" variant="outline" disabled>
            {t('common.cancel')}
          </Button>
        </div>
      </form>
    </>
  );
}
