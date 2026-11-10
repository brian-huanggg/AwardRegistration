import { useId, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Download } from 'lucide-react';
import { ApiError } from '@/shared/api';
import { SLIDE_EXTS } from '@/shared/slides';
import { Button } from '@/shared/ui/button';
import { CopyButton } from '@/shared/ui/copy-button';
import { Chip } from '@/shared/ui/chip';
import { PageHero } from '@/shared/ui/page-hero';
import heroImage from '@/shared/assets/hero-landscape.webp';
import { fetchIdea, fetchSlides, IncompleteDownloadError, ideaKey, saveBlob, type IdeaDetail as Idea } from './detail';

/** Section heading (Pencil uwwe8): bold uppercase, ink. */
const SectionTitle = ({ id, children }: { id?: string; children: React.ReactNode }) => (
  <h2 id={id} className="text-xl font-bold tracking-[0.02em] text-brand-ink uppercase sm:text-[22px]">
    {children}
  </h2>
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
  /** Edit, delete or leave, right of the "Idea summary" heading. */
  headerActions?: (idea: Idea) => React.ReactNode;
  /** Extra controls in the slides card (replace). */
  slideActions?: (idea: Idea) => React.ReactNode;
  /** Extra chips in the hero after the relation chip (the seat count). */
  heroChips?: (idea: Idea) => React.ReactNode;
  /** The side column: team and join requests. */
  side?: (idea: Idea) => React.ReactNode;
  /** The deadline alert, last on narrow screens, under the slides on wide ones. */
  notice?: React.ReactNode;
}

/**
 * Idea detail page body (UI-04, Eco restyle E3: Pencil `uwwe8`). A full-bleed hero (back link,
 * category · org · direction eyebrow, h1, relation and seat chips), then two columns from 1024 px:
 * Idea content left, team right; one column below in DOM order hero → summary → slides → team →
 * requests → notice.
 */
export function IdeaDetail({ id, me, notFound, heading, back, headerActions, slideActions, heroChips, side, notice }: Props) {
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
  const summaryId = useId();
  const slidesId = useId();

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

  const leader = data.leader.empNo === me.empNo;
  const relation = admin
    ? { tone: 'neutral' as const, text: t('nav.administrator') }
    : leader
      ? { tone: 'lime' as const, text: t('pages.ideaDetail.youAreLeader') }
      : data.members.some((m) => m.empNo === me.empNo)
        ? { tone: 'lavender' as const, text: t('pages.ideaDetail.youAreMember') }
        : null;
  const category = t(`enums.category.${data.category}`);
  const org = t(`enums.org.${data.org}`);
  const direction = t(`enums.direction.${data.direction}`);
  // the filename prefix is the first part of the expected name ({I|O}_…), as the server builds it
  const prefix = data.expectedFilenameBase.split('_')[0];

  const rows: [string, React.ReactNode][] = [
    [t('fields.category'), prefix ? `${category} (${prefix})` : category],
    [t('fields.org'), org],
    [t('fields.direction'), direction],
    [t('fields.leader'), `${data.leader.name} · ${data.leader.empNo}`],
    [t('pages.ideaDetail.createdAt'), when(data.createdAt)],
    [t('pages.ideaDetail.updatedAt'), when(data.updatedAt)],
  ];

  return (
    <>
      <PageHero
        className="-mx-4 -mt-4 sm:-mx-8 sm:-mt-8 xl:-mx-14"
        image={heroImage}
        title={
          // back link and eyebrow live in the title slot so the DOM order is back → eyebrow → h1
          <div className="flex flex-col gap-2.5">
            <div className="mb-1">{back}</div>
            <p className="text-base tracking-[0.06em] text-brand-purple uppercase">
              {category} · {org} · {direction}
            </p>
            {heading(data.name)}
          </div>
        }
        actions={
          <>
            {relation && <Chip tone={relation.tone}>{relation.text}</Chip>}
            {heroChips?.(data)}
          </>
        }
      />

      <div className="mt-6 grid items-start gap-8 sm:mt-9 lg:grid-cols-[minmax(0,1fr)_360px] lg:grid-rows-[auto_1fr] lg:gap-10 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="min-w-0 space-y-8 lg:col-start-1 lg:row-start-1">
          <section aria-labelledby={summaryId}>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-3">
              <SectionTitle id={summaryId}>{t('pages.ideaDetail.details')}</SectionTitle>
              <div aria-hidden="true" className="hidden h-px flex-1 sm:block" />
              <div className="flex flex-wrap gap-2.5">{headerActions?.(data)}</div>
            </div>
            <p className="mt-5 text-base leading-7 break-words whitespace-pre-wrap text-foreground sm:text-[17px]">{data.summary}</p>
            <dl className="mt-6 grid border-b sm:grid-cols-2 lg:grid-cols-3">
              {rows.map(([k, v]) => (
                <div key={k} className="min-w-0 border-t py-4 pr-6">
                  <dt className="text-[11px] font-semibold tracking-[0.13em] text-muted-foreground uppercase">{k}</dt>
                  <dd className="mt-1.5 text-base font-medium break-words text-brand-ink">{v}</dd>
                </div>
              ))}
            </dl>
          </section>

          <section aria-labelledby={slidesId}>
            <SectionTitle id={slidesId}>{t('pages.ideaDetail.slides')}</SectionTitle>
            <p className="mt-1.5 text-sm text-muted-foreground">{t('pages.ideaDetail.slidesHint')}</p>
            <div className="mt-4 flex flex-wrap items-center gap-4 rounded-[18px] bg-brand-lavender p-4 sm:px-5">
              <span aria-hidden="true" className="flex size-12 shrink-0 items-center justify-center rounded-[10px] bg-brand-purple text-xs font-bold tracking-[0.08em] text-white uppercase">
                {data.slides.ext}
              </span>
              <div className="min-w-[12rem] flex-1">
                <p className="text-[15px] font-semibold break-all text-brand-ink">{data.slides.downloadFilename}</p>
                <p className="mt-1 text-[13px] text-muted-foreground">
                  {data.slides.ext.toUpperCase()} · {t('pages.ideaDetail.size', { kb: Math.max(1, Math.round(data.slides.size / 1024)) })} ·{' '}
                  {t('pages.ideaDetail.uploadedAt')} {when(data.slides.updatedAt)}
                </p>
              </div>
              <div className="flex flex-wrap gap-2.5">
                {slideActions?.(data)}
                <Button variant="lime" size="pill" disabled={busy} onClick={() => void download()}>
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
            <div className="mt-4 text-[13px] text-muted-foreground">
              <h3 className="font-medium">{t('pages.ideaForm.expectedTitle')}</h3>
              <ul className="mt-1.5 flex flex-col gap-1.5">
                {SLIDE_EXTS.map((ext) => {
                  const name = `${data.expectedFilenameBase}.${ext}`;
                  return (
                    <li key={ext} className="flex items-center gap-2">
                      <code className="min-w-0 flex-1 rounded-md bg-field px-2 py-1 break-all text-foreground">{name}</code>
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
          </section>
        </div>

        <div className="min-w-0 space-y-6 lg:col-start-2 lg:row-span-2 lg:row-start-1">{side?.(data)}</div>

        {notice && <div className="min-w-0 lg:col-start-1 lg:row-start-2">{notice}</div>}
      </div>
    </>
  );
}
