import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router';
import { deadlineKey, useDeadline } from '@/features/deadline/useDeadline';
import { CreateIdeaForm } from '@/features/idea/CreateIdeaForm';
import { IdeaDetail as IdeaDetailView } from '@/features/idea/IdeaDetail';
import { IdeaList as IdeaListView } from '@/features/idea/IdeaList';
import { Button } from '@/shared/ui/button';
import { Field } from '@/shared/ui/form-field';
import { PageHeading } from '../layouts';
import { NotFound } from './NotFound';
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
      {me && (
        <IdeaListView
          admin={me.role === 'ADMIN'}
          actions={
            me.role === 'USER' && (
              <Button asChild variant="outline">
                <Link to="/ideas/new">{t('pages.ideaList.create')}</Link>
              </Button>
            )
          }
        />
      )}
    </>
  );
}

export function IdeaCreate() {
  const { t } = useTranslation();
  const { data: me } = useMe();
  const { data: deadline } = useDeadline();
  const client = useQueryClient();
  const navigate = useNavigate();
  const open = deadline?.state === 'OPEN'; // Frontend.md §6.3 rule 6: a hint and disabled controls, not a guard
  return (
    <>
      <PageHeading title={t('pages.ideaCreate.title')} />
      <div className="max-w-2xl rounded-xl border bg-card p-6">
        {me && (
          <CreateIdeaForm
            leader={{ name: me.name, empNo: me.empNo }}
            canSubmit={open}
            onOutcomeUnknown={() => void client.invalidateQueries({ queryKey: ['ideas'] })}
            onSubmissionRejected={() => void client.invalidateQueries({ queryKey: deadlineKey })}
            onCreated={(idea) => {
              // the response is the detail: cache it, no follow-up GET (Frontend.md §6.7)
              client.setQueryData(['idea', idea.id], idea);
              void client.invalidateQueries({ queryKey: ['ideas'] });
              void navigate(`/ideas/${idea.id}`);
            }}
          />
        )}
      </div>
    </>
  );
}

export function IdeaDetail() {
  const { t } = useTranslation();
  const { id = '' } = useParams();
  const { data: me } = useMe();
  return (
    <>
      {me && (
        <IdeaDetailView id={id} admin={me.role === 'ADMIN'} notFound={<NotFound />} heading={<PageHeading title={t('pages.ideaDetail.title')} />} />
      )}
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
