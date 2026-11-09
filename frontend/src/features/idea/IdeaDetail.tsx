import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { ApiError } from '@/shared/api';
import { SLIDE_EXTS } from '@/shared/slides';
import { Button } from '@/shared/ui/button';
import { CopyButton } from '@/shared/ui/copy-button';
import { fetchIdea, fetchSlides, IncompleteDownloadError, ideaKey, saveBlob, type IdeaDetail as Idea } from './detail';

const Section = ({ title, children }: { title: string; children?: React.ReactNode }) => (
  <section className="mb-4 rounded-xl border bg-card p-6">
    <h2 className="mb-3 text-lg font-semibold">{title}</h2>
    {children}
  </section>
);

interface Props {
  id: string;
  admin: boolean;
  /** Rendered instead of the page when the server answers 404 (the one uniform 404 page). */
  notFound: React.ReactNode;
  /** The page heading; left out for the 404 page, which has its own (one h1). */
  heading: React.ReactNode;
  /** Extra controls in the slides section (replace, delete: later PRs). */
  /** The Leader's link to the edit page, above the details. */
  editAction?: (idea: Idea) => React.ReactNode;
  slideActions?: (idea: Idea) => React.ReactNode;
  /** Per-row controls: remove an approved Member, approve or reject a pending request, leave (own row). */
  memberActions?: (idea: Idea, member: Idea['members'][number]) => React.ReactNode;
  pendingActions?: (idea: Idea, request: NonNullable<Idea['pendingRequests']>[number]) => React.ReactNode;
}

export function IdeaDetail({ id, admin, notFound, heading, editAction, slideActions, memberActions, pendingActions }: Props) {
  const { t, i18n } = useTranslation();
  // An Admin's GET writes an IDEA_VIEW audit row (Frontend.md §6.7): no refocus refetch, no polling.
  const { data, isPending, error, refetch } = useQuery({
    queryKey: ideaKey(id),
    queryFn: ({ signal }) => fetchIdea(id, signal),
    retry: false,
    staleTime: 30_000, // the create response is cached as the detail: no follow-up GET
    refetchOnWindowFocus: !admin,
  });
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  if (isPending && !error)
    return (
      <>
        {heading}
        <p role="status">{t('common.loading')}</p>
      </>
    );
  if (error instanceof ApiError && error.status === 404) return <>{notFound}</>;
  if (!data) {
    const requestId = error instanceof ApiError && error.requestId ? ` ${t('common.requestId', { id: error.requestId })}` : '';
    const key = error instanceof ApiError && i18n.exists(`errors.${error.code}`) ? error.code : 'UNKNOWN';
    return (
      <>
        {heading}
        <div role="alert">
          <p className="mb-2">{t(`errors.${key}`) + requestId}</p>
          <Button variant="outline" onClick={() => void refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      </>
    );
  }

  const fmtDate = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' });
  const when = (iso: string) => <time dateTime={iso}>{fmtDate.format(new Date(iso))}</time>;
  const person = (p: { name: string; empNo: string }) => `${p.name} (${p.empNo})`;

  const download = async () => {
    setBusy(true);
    setFailure(null);
    try {
      const { blob, filename } = await fetchSlides(id, data.slides.downloadFilename);
      saveBlob(blob, filename);
    } catch (e) {
      if (e instanceof IncompleteDownloadError) setFailure(t('pages.ideaDetail.downloadIncomplete'));
      else if (e instanceof ApiError) {
        const key = i18n.exists(`errors.${e.code}`) ? e.code : 'UNKNOWN';
        setFailure(t(`errors.${key}`) + (e.requestId ? ` ${t('common.requestId', { id: e.requestId })}` : ''));
      } else setFailure(t('pages.ideaDetail.downloadIncomplete'));
    } finally {
      setBusy(false);
    }
  };

  const rows: [string, React.ReactNode][] = [
    [t('fields.ideaName'), data.name],
    [t('fields.category'), t(`enums.category.${data.category}`)],
    [t('fields.org'), t(`enums.org.${data.org}`)],
    [t('fields.direction'), t(`enums.direction.${data.direction}`)],
    [t('fields.leader'), person(data.leader)],
    [t('fields.summary'), <span key="s" className="whitespace-pre-wrap">{data.summary}</span>],
    [t('pages.ideaDetail.createdAt'), when(data.createdAt)],
  ];

  return (
    <>
      {heading}
      {editAction && <div className="mb-4">{editAction(data)}</div>}
      <Section title={t('pages.ideaDetail.details')}>
        <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
          {rows.map(([k, v]) => (
            <div key={k} className="contents">
              <dt className="font-medium">{k}</dt>
              <dd className="min-w-0 break-words">{v}</dd>
            </div>
          ))}
        </dl>
      </Section>
      <Section title={t('pages.ideaDetail.members')}>
        {data.members.length === 0 ? (
          <p className="text-sm">{t('pages.ideaDetail.noMembers')}</p>
        ) : (
          <ul className="text-sm">
            {data.members.map((m) => (
              <li key={m.empNo} className="flex flex-wrap items-center gap-2 py-1">
                <span>{person(m)}</span>
                {memberActions?.(data, m)}
              </li>
            ))}
          </ul>
        )}
      </Section>
      {data.pendingRequests !== null && (
        <Section title={t('pages.ideaDetail.pending')}>
          {data.pendingRequests.length === 0 ? (
            <p className="text-sm">{t('pages.ideaDetail.noPending')}</p>
          ) : (
            <ul className="text-sm">
              {data.pendingRequests.map((r) => (
                <li key={r.empNo} className="flex flex-wrap items-center gap-2 py-1">
                  <span>
                    {person(r)} · {when(r.requestedAt)}
                  </span>
                  {pendingActions?.(data, r)}
                </li>
              ))}
            </ul>
          )}
        </Section>
      )}
      <Section title={t('pages.ideaDetail.slides')}>
        <p className="mb-3 text-sm">
          {data.slides.downloadFilename} · {t('pages.ideaDetail.size', { kb: Math.max(1, Math.round(data.slides.size / 1024)) })} ·{' '}
          {t('pages.ideaDetail.updatedAt')} {when(data.slides.updatedAt)}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" disabled={busy} onClick={() => void download()}>
            {busy ? t('pages.ideaDetail.downloading') : t('pages.ideaDetail.download')}
          </Button>
          {slideActions?.(data)}
        </div>
        {failure && (
          <p role="alert" className="mt-3 text-sm text-destructive">
            {failure}
          </p>
        )}
        <h3 className="mt-4 text-sm font-medium">{t('pages.ideaForm.expectedTitle')}</h3>
        <ul className="mt-1 flex flex-col gap-2 text-sm">
          {SLIDE_EXTS.map((ext) => {
            const name = `${data.expectedFilenameBase}.${ext}`;
            return (
              <li key={ext} className="flex items-center gap-2">
                <code className="min-w-0 flex-1 break-all rounded bg-muted px-2 py-1">{name}</code>
                <CopyButton
                  value={name}
                  label={t('pages.ideaForm.copy', { ext: ext.toUpperCase() })}
                  copiedText={t('pages.ideaForm.copied')}
                  failedText={t('pages.ideaForm.copyFailed')}
                />
              </li>
            );
          })}
        </ul>
      </Section>
    </>
  );
}
