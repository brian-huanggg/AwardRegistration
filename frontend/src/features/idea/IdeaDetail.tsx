import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Download, FileText, Info } from 'lucide-react';
import { ApiError } from '@/shared/api';
import { SLIDE_EXTS } from '@/shared/slides';
import { Button } from '@/shared/ui/button';
import { CopyButton } from '@/shared/ui/copy-button';
import { cn } from '@/shared/ui/utils';
import { fetchIdea, fetchSlides, IncompleteDownloadError, ideaKey, saveBlob, type IdeaDetail as Idea } from './detail';

const Card = ({ title, children }: { title: string; children?: React.ReactNode }) => (
  <section className="rounded-xl border bg-card p-5 text-card-foreground shadow-sm sm:p-6">
    <h2 className="mb-3 text-base font-semibold">{title}</h2>
    {children}
  </section>
);

const Badge = ({ tone = 'outline', children }: { tone?: 'primary' | 'outline'; children: React.ReactNode }) => (
  <span
    className={cn(
      'inline-flex max-w-full items-center rounded-full px-2.5 py-0.5 text-xs font-semibold',
      tone === 'primary' ? 'bg-primary text-primary-foreground' : 'border text-foreground',
    )}
  >
    {children}
  </span>
);

interface Props {
  id: string;
  me: { empNo: string; role: 'USER' | 'ADMIN' };
  /** Rendered instead of the page when the server answers 404 (the one uniform 404 page). */
  notFound: React.ReactNode;
  /** The page h1: the Idea name once loaded, a generic title before. Left out for the 404 page (one h1). */
  heading: (title?: string) => React.ReactNode;
  /** The back link above the heading. */
  back?: React.ReactNode;
  /** Edit, delete or leave, right of the heading. */
  headerActions?: (idea: Idea) => React.ReactNode;
  /** Extra controls in the slides card (replace). */
  slideActions?: (idea: Idea) => React.ReactNode;
  /** The side column: team and join requests. */
  side?: (idea: Idea) => React.ReactNode;
  /** The deadline alert, last on narrow screens, under the slides on wide ones. */
  notice?: React.ReactNode;
}

/**
 * Idea detail page body (UI-04, Pencil `rsQOw`). Two columns from 1024 px: Idea content left,
 * team right; one column below in DOM order header → summary → slides → team → requests → notice.
 */
export function IdeaDetail({ id, me, notFound, heading, back, headerActions, slideActions, side, notice }: Props) {
  const { t, i18n } = useTranslation();
  const admin = me.role === 'ADMIN';
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
        {back}
        {heading()}
        <p role="status">{t('common.loading')}</p>
      </>
    );
  if (error instanceof ApiError && error.status === 404) return <>{notFound}</>;
  if (!data) {
    const requestId = error instanceof ApiError && error.requestId ? ` ${t('common.requestId', { id: error.requestId })}` : '';
    const key = error instanceof ApiError && i18n.exists(`errors.${error.code}`) ? error.code : 'UNKNOWN';
    return (
      <>
        {back}
        {heading()}
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

  const relation = admin
    ? t('nav.administrator')
    : data.leader.empNo === me.empNo
      ? t('pages.ideaDetail.youAreLeader')
      : data.members.some((m) => m.empNo === me.empNo)
        ? t('pages.ideaDetail.youAreMember')
        : null;
  const category = t(`enums.category.${data.category}`);
  // the filename prefix is the first part of the expected name ({I|O}_…), as the server builds it
  const prefix = data.expectedFilenameBase.split('_')[0];

  const rows: [string, React.ReactNode][] = [
    [t('fields.category'), prefix ? `${category} (${prefix})` : category],
    [t('fields.org'), t(`enums.org.${data.org}`)],
    [t('fields.direction'), t(`enums.direction.${data.direction}`)],
    [t('fields.leader'), `${data.leader.name} · ${data.leader.empNo}`],
    [t('pages.ideaDetail.createdAt'), when(data.createdAt)],
    [t('pages.ideaDetail.updatedAt'), when(data.updatedAt)],
  ];

  return (
    <>
      <div className="space-y-3">
        {back}
        <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
          <div className="min-w-0 space-y-3 break-words">
            {heading(data.name)}
            <div className="flex flex-wrap gap-2">
              {relation && <Badge tone="primary">{relation}</Badge>}
              <Badge>{category}</Badge>
              <Badge>{t(`enums.org.${data.org}`)}</Badge>
              <Badge>{t(`enums.direction.${data.direction}`)}</Badge>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">{headerActions?.(data)}</div>
        </div>
      </div>

      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_360px] lg:grid-rows-[auto_1fr]">
        <div className="min-w-0 space-y-6 lg:col-start-1 lg:row-start-1">
          <Card title={t('pages.ideaDetail.details')}>
            <p className="whitespace-pre-wrap break-words text-sm">{data.summary}</p>
            <dl className="mt-5 grid gap-x-6 gap-y-4 border-t pt-5 text-sm sm:grid-cols-2 lg:grid-cols-3">
              {rows.map(([k, v]) => (
                <div key={k} className="min-w-0">
                  <dt className="text-xs text-muted-foreground">{k}</dt>
                  <dd className="mt-1 break-words font-medium">{v}</dd>
                </div>
              ))}
            </dl>
          </Card>

          <Card title={t('pages.ideaDetail.slides')}>
            <p className="mb-4 text-sm text-muted-foreground">{t('pages.ideaDetail.slidesHint')}</p>
            <div className="flex flex-wrap items-center gap-3 rounded-lg border p-3">
              <FileText aria-hidden="true" className="size-8 shrink-0 text-muted-foreground" />
              <div className="min-w-0 flex-1">
                <p className="break-all text-sm font-medium">{data.slides.downloadFilename}</p>
                <p className="text-xs text-muted-foreground">
                  {data.slides.ext.toUpperCase()} · {t('pages.ideaDetail.size', { kb: Math.max(1, Math.round(data.slides.size / 1024)) })} ·{' '}
                  {t('pages.ideaDetail.uploadedAt')} {when(data.slides.updatedAt)}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                {slideActions?.(data)}
                <Button size="sm" disabled={busy} onClick={() => void download()}>
                  <Download aria-hidden="true" />
                  {busy ? t('pages.ideaDetail.downloading') : t('pages.ideaDetail.download')}
                </Button>
              </div>
            </div>
            {failure && (
              <p role="alert" className="mt-3 text-sm text-destructive">
                {failure}
              </p>
            )}
            <div className="mt-4 flex gap-2 text-sm">
              <Info aria-hidden="true" className="mt-1.5 size-4 shrink-0 text-muted-foreground" />
              <div className="min-w-0 flex-1">
                <h3 className="font-medium">{t('pages.ideaForm.expectedTitle')}</h3>
                <ul className="mt-1 flex flex-col gap-2">
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
              </div>
            </div>
          </Card>
        </div>

        <div className="min-w-0 space-y-6 lg:col-start-2 lg:row-span-2 lg:row-start-1">{side?.(data)}</div>

        {notice && <div className="min-w-0 lg:col-start-1 lg:row-start-2">{notice}</div>}
      </div>
    </>
  );
}
