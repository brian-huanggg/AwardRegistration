import { useEffect, useState, type ReactNode } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ArrowRight, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
import { Chip } from '@/shared/ui/chip';
import { Input } from '@/shared/ui/input';
import { NativeSelect } from '@/shared/ui/native-select';
import { cn } from '@/shared/ui/utils';
import { CATEGORIES, DIRECTIONS, ORGS } from './create';
import { DateRangeFilter, FacetFilter } from './FacetFilter';
import {
  defaultOrder,
  fetchIdeaPage,
  hasFilters,
  ideasKey,
  parseListParams,
  SIZES,
  toSearchParams,
  type IdeaSummary,
  type ListParams,
} from './list';
import { SortMenu } from './SortMenu';
import { useWide } from './useWide';

const DEBOUNCE_MS = 300;
/** A team is the Leader plus up to 3 Members (PRD). */
export const MAX_TEAM = 4;

/**
 * Figures for the page hero, taken from the list response only (no extra request): the unfiltered
 * total, and for a User the number of Ideas they lead when every Idea is on the current page.
 * Undefined until known; kept while filters are applied so the hero does not jump.
 */
export interface ListStats {
  total?: number;
  led?: number;
}

interface Props {
  /** Admin sees every Idea and can filter by Leader; a User sees the own ones (the server decides, `can()` shows). */
  admin: boolean;
  /** Admin row actions (F6-01), built by the page from `can()`; null hides them for that row. */
  rowActions?: (idea: IdeaSummary) => React.ReactNode;
  /** The page hero, rendered above the toolbar with the figures this list knows. */
  hero?: (stats: ListStats) => ReactNode;
  /** Page actions at the right end of the toolbar (User: Join a team, New Idea). */
  actions?: ReactNode;
}

/** Badge text, never colour alone: Leader lime, Member lavender (design `vF25U`). */
function RoleChip({ relation }: { relation: 'LEADER' | 'MEMBER' }) {
  const { t } = useTranslation();
  return <Chip tone={relation === 'LEADER' ? 'lime' : 'lavender'}>{t(`pages.ideaList.relation.${relation}`)}</Chip>;
}

/** 1 … 4 5 6 … 12: first, last and the neighbours of the current page. */
function pageList(current: number, pages: number): (number | 'gap')[] {
  if (pages <= 7) return Array.from({ length: pages }, (_, i) => i + 1);
  const keep = [...new Set([1, current - 1, current, current + 1, pages])].filter((p) => p >= 1 && p <= pages).sort((a, b) => a - b);
  const out: (number | 'gap')[] = [];
  keep.forEach((p, i) => {
    const prev = keep[i - 1];
    if (prev !== undefined && p - prev > 1) out.push('gap');
    out.push(p);
  });
  return out;
}

const PILL_INPUT = 'h-11 rounded-full border-[1.5px] border-border text-sm shadow-none';

export function IdeaList({ admin, rowActions, hero, actions }: Props) {
  const { t, i18n } = useTranslation();
  const [sp, setSp] = useSearchParams();
  const params = parseListParams(sp);
  const wide = useWide();

  // `replace` for typing and filter changes keeps Back from stepping through every keystroke; paging pushes.
  const update = (patch: Partial<ListParams>, { push = false } = {}) => {
    const next = { ...params, page: 1, ...patch };
    if (patch.sort && patch.order === undefined) next.order = defaultOrder(patch.sort);
    setSp(toSearchParams(next), { replace: !push });
  };

  // `q` is debounced: the box keeps its own text, the URL follows 300 ms after the last key.
  const [text, setText] = useState(params.q);
  const [seenQ, setSeenQ] = useState(params.q);
  if (seenQ !== params.q) {
    // the URL changed from outside (Back, a link, "clear"): the box follows
    setSeenQ(params.q);
    setText(params.q);
  }
  useEffect(() => {
    if (text === params.q) return;
    const id = setTimeout(() => update({ q: text }), DEBOUNCE_MS);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `update` is rebuilt every render from the same URL
  }, [text, params.q]);

  const { data, isPending, isError, error, refetch, isPlaceholderData } = useQuery({
    queryKey: ideasKey(params),
    queryFn: ({ signal }) => fetchIdeaPage(params, signal),
    placeholderData: keepPreviousData,
    retry: false,
  });

  const pages = data ? Math.max(1, Math.ceil(data.total / data.size)) : 1;
  // A URL beyond the last page (items deleted meanwhile, or hand-edited) goes to the last one.
  useEffect(() => {
    if (data && !isPlaceholderData && data.total > 0 && params.page > pages) update({ page: pages });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, isPlaceholderData, pages, params.page]);

  const filtered = hasFilters(params);

  // Hero figures: only from an unfiltered response; "led by you" only when the whole list is on this page.
  const [known, setKnown] = useState<ListStats>({});
  if (data && !isPlaceholderData && !filtered) {
    const complete = !admin && data.page === 1 && data.items.length >= data.total;
    const led = complete ? data.items.filter((i) => i.myRelation === 'LEADER').length : known.total === data.total ? known.led : undefined;
    if (known.total !== data.total || known.led !== led) setKnown({ total: data.total, led });
  }

  const fmt = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' });
  const clear = () => {
    setText('');
    setSp(toSearchParams({ ...params, page: 1, q: '', category: '', org: '', direction: '', leaderEmpNo: '', from: '', to: '' }), { replace: true });
  };
  const options = (name: 'category' | 'org' | 'direction', list: readonly string[]) => list.map((v) => ({ value: v, label: t(`enums.${name}.${v}`) }));
  const caption = admin ? t('nav.registrations') : t('nav.myIdeas');
  const size = (i: IdeaSummary) => i.memberCount + 1; // the Leader has no member row
  const team = (i: IdeaSummary) => (
    <span className={cn('tabular-nums', size(i) >= MAX_TEAM && admin && 'font-semibold text-brand-forest')}>{`${size(i)}/${MAX_TEAM}`}</span>
  );
  const created = (i: IdeaSummary) => <time dateTime={i.createdAt}>{fmt.format(new Date(i.createdAt))}</time>;
  const category = (i: IdeaSummary) => t(`enums.category.${i.category}`, { defaultValue: i.category });
  const org = (i: IdeaSummary) => t(`enums.org.${i.org}`, { defaultValue: i.org });
  const actionsColumn = admin && rowActions !== undefined;

  const th = (label: React.ReactNode, className?: string, sortKey?: ListParams['sort']) => (
    <th
      scope="col"
      aria-sort={sortKey && params.sort === sortKey ? (params.order === 'asc' ? 'ascending' : 'descending') : undefined}
      className={cn(
        'h-10 border-b-[1.5px] border-foreground px-4 text-left align-middle text-[11px] font-semibold tracking-[0.13em] whitespace-nowrap text-muted-foreground uppercase',
        className,
      )}
    >
      {label}
    </th>
  );
  // Separate borders so each row can be a rounded lavender band on hover / focus (design row state).
  const td = 'border-b border-border px-4 py-5 align-middle group-hover:bg-brand-lavender group-focus-within:bg-brand-lavender first:rounded-l-[14px] last:rounded-r-[14px]';
  const narrowHidden = 'hidden lg:table-cell'; // 768–1023 px: no ESG direction and Created columns

  const from = data && data.total > 0 ? (data.page - 1) * data.size + 1 : 0;
  const to = data ? Math.min(data.total, (data.page - 1) * data.size + data.items.length) : 0;
  const footerExtra =
    filtered && known.total !== undefined
      ? t('pages.ideaList.allTotal', { count: known.total })
      : !admin && !filtered && known.led !== undefined
        ? t('pages.ideaList.ledByYou', { count: known.led })
        : null;
  const go = (page: number) => update({ page }, { push: true });

  return (
    <>
      {hero?.(known)}

      <div className="mt-7 mb-5 flex flex-wrap items-center gap-2.5">
        <form role="search" aria-label={t('pages.ideaList.filters')} onSubmit={(e) => e.preventDefault()} className="flex min-w-0 flex-1 flex-wrap items-center gap-2.5">
          <div className="relative w-full sm:w-[300px]">
            <label htmlFor="q" className="sr-only">
              {t('pages.ideaList.search')}
            </label>
            <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-4 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="q"
              name="q"
              type="search"
              className={cn(PILL_INPUT, 'bg-field pl-10')}
              placeholder={t('pages.ideaList.searchPlaceholder')}
              value={text}
              onChange={(e) => setText(e.target.value)}
            />
          </div>
          <FacetFilter label={t('fields.org')} value={params.org} options={options('org', ORGS)} onChange={(org) => update({ org })} />
          <FacetFilter label={t('fields.category')} value={params.category} options={options('category', CATEGORIES)} onChange={(category) => update({ category })} />
          <FacetFilter label={t('fields.direction')} value={params.direction} options={options('direction', DIRECTIONS)} onChange={(direction) => update({ direction })} />
          {admin && (
            <div className="w-44">
              <label htmlFor="leaderEmpNo" className="sr-only">
                {t('fields.leaderEmpNo')}
              </label>
              <Input
                id="leaderEmpNo"
                name="leaderEmpNo"
                className={cn(PILL_INPUT, 'bg-background px-4 placeholder:text-xs placeholder:font-semibold placeholder:tracking-[0.1em] placeholder:uppercase')}
                inputMode="numeric"
                maxLength={6}
                placeholder={t('fields.leaderEmpNo')}
                value={params.leaderEmpNo}
                onChange={(e) => update({ leaderEmpNo: e.target.value.replace(/\D/g, '') })}
              />
            </div>
          )}
          <DateRangeFilter label={t('pages.ideaList.created')} from={params.from} to={params.to} onChange={(patch) => update(patch)} />
          {filtered && (
            <Button type="button" variant="ghost" size="pill-sm" className="tracking-[0.1em] uppercase" onClick={clear}>
              {t('pages.ideaList.clear')}
            </Button>
          )}
          <div className="ml-auto">
            <SortMenu sort={params.sort} order={params.order} onChange={(v) => update(v)} />
          </div>
        </form>
        {actions && <div className="flex flex-wrap gap-2.5">{actions}</div>}
      </div>

      <div aria-live="polite" role="status" className="sr-only">
        {data && t('pages.ideaList.total', { count: data.total })}
      </div>

      {isPending && <p role="status">{t('common.loading')}</p>}
      {isError && (
        <div role="alert" className="rounded-[14px] border border-destructive p-4">
          <p>{t(`errors.${error instanceof ApiError && error.code !== 'UNKNOWN' ? error.code : 'UNKNOWN'}`)}</p>
          {error instanceof ApiError && error.requestId && <p className="text-sm">{t('common.requestId', { id: error.requestId })}</p>}
          <Button type="button" variant="light" size="pill-sm" className="mt-3" onClick={() => void refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      )}

      {data && data.items.length === 0 && (
        <p className="mt-4 rounded-[14px] border bg-card p-8 text-center text-muted-foreground">{filtered ? t('pages.ideaList.noMatches') : t('pages.ideaList.empty')}</p>
      )}

      {data && data.items.length > 0 && (
        <div className={isPlaceholderData ? 'opacity-60' : undefined} aria-busy={isPlaceholderData}>
          {wide ? (
            <div className="overflow-x-auto">
              <table className="w-full border-separate border-spacing-0 text-sm text-foreground">
                <caption className="sr-only">{caption}</caption>
                <thead>
                  <tr>
                    {th(t('fields.ideaName'), undefined, 'name')}
                    {admin ? th(t('fields.leader')) : th(t('pages.ideaList.role'))}
                    {th(t('fields.category'))}
                    {th(t('fields.org'))}
                    {th(t('fields.direction'), narrowHidden)}
                    {th(t('pages.ideaList.team'))}
                    {th(t('pages.ideaList.created'), narrowHidden, 'createdAt')}
                    {actionsColumn ? th(t('pages.ideaList.actions'), 'text-right') : !admin && <td aria-hidden="true" className="w-12 border-b-[1.5px] border-foreground" />}
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((i) => (
                    <tr key={i.id} className="group">
                      <th scope="row" className={cn(td, 'text-left font-semibold')}>
                        <Link to={`/ideas/${i.id}`} className="text-base text-brand-ink underline-offset-4 hover:underline focus-visible:underline">
                          {i.name}
                        </Link>
                      </th>
                      {admin ? (
                        <td className={td}>
                          <span className="block whitespace-nowrap">{i.leader.name}</span>
                          <span className="block text-xs tracking-[0.04em] text-muted-foreground tabular-nums">{i.leader.empNo}</span>
                        </td>
                      ) : (
                        <td className={td}>{i.myRelation && <RoleChip relation={i.myRelation} />}</td>
                      )}
                      <td className={td}>{category(i)}</td>
                      <td className={cn(td, 'font-semibold tracking-[0.06em]')}>{org(i)}</td>
                      <td className={cn(td, narrowHidden)}>{t(`enums.direction.${i.direction}`, { defaultValue: i.direction })}</td>
                      <td className={cn(td, 'text-[13px] font-medium text-muted-foreground')}>{team(i)}</td>
                      <td className={cn(td, narrowHidden, 'whitespace-nowrap text-muted-foreground')}>{created(i)}</td>
                      {actionsColumn ? (
                        <td className={cn(td, 'text-right')}>{rowActions(i)}</td>
                      ) : (
                        !admin && (
                          <td className={cn(td, 'text-right')}>
                            {/* the name link is the row's tab stop; this arrow is a pointer-only shortcut to the same page */}
                            <Link to={`/ideas/${i.id}`} tabIndex={-1} aria-hidden="true" className="inline-flex text-brand-ink">
                              <ArrowRight className="size-[18px]" />
                            </Link>
                          </td>
                        )
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <ul aria-label={caption} className="flex flex-col gap-3">
              {data.items.map((i) => (
                <li key={i.id} className="rounded-[14px] border bg-card p-4 text-sm">
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <Link to={`/ideas/${i.id}`} className="min-w-0 text-base font-semibold break-words text-brand-ink underline-offset-4 hover:underline">
                      {i.name}
                    </Link>
                    {!admin && i.myRelation && <RoleChip relation={i.myRelation} />}
                  </div>
                  <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-2 text-muted-foreground">
                    {admin && (
                      <span>
                        {t('fields.leader')}: {i.leader.name} ({i.leader.empNo})
                      </span>
                    )}
                    <span>{category(i)}</span>
                    <span className="font-semibold tracking-[0.06em] text-foreground">{org(i)}</span>
                    <span>
                      {t('pages.ideaList.team')} {team(i)}
                    </span>
                  </div>
                  {actionsColumn && <div className="mt-3 flex justify-end">{rowActions(i)}</div>}
                </li>
              ))}
            </ul>
          )}

          <nav aria-label={t('pages.ideaList.paging')} className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-3 text-[13px] text-muted-foreground">
            <p>
              {t('pages.ideaList.showing', { from, to, total: data.total })}
              {footerExtra && ` · ${footerExtra}`}
            </p>
            <label className="flex items-center gap-2 sm:ml-auto">
              {t('pages.ideaList.perPage')}
              <span className="w-20">
                <NativeSelect value={params.size} onChange={(e) => update({ size: Number(e.target.value) })} className="h-9 rounded-full text-foreground">
                  {SIZES.map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
                </NativeSelect>
              </span>
            </label>
            <div className="flex flex-wrap items-center gap-2">
              <Button type="button" variant="light" size="pill-icon" className="size-9" aria-label={t('pages.ideaList.prev')} disabled={params.page <= 1} onClick={() => go(params.page - 1)}>
                <ChevronLeft aria-hidden="true" />
              </Button>
              {pageList(data.page, pages).map((p, idx) =>
                p === 'gap' ? (
                  <span key={`gap-${idx}`} aria-hidden="true" className="px-1 text-foreground">
                    …
                  </span>
                ) : (
                  <Button
                    key={p}
                    type="button"
                    variant="ghost"
                    size="pill-icon"
                    aria-label={t('pages.ideaList.goToPage', { page: p })}
                    aria-current={p === data.page ? 'page' : undefined}
                    className={cn('size-9 text-[13px] font-semibold text-foreground tabular-nums', p === data.page && 'bg-brand-ink text-background hover:bg-brand-ink/90 hover:text-background')}
                    onClick={() => p !== data.page && go(p)}
                  >
                    {p}
                  </Button>
                ),
              )}
              <Button type="button" variant="light" size="pill-icon" className="size-9" aria-label={t('pages.ideaList.next')} disabled={params.page >= pages} onClick={() => go(params.page + 1)}>
                <ChevronRight aria-hidden="true" />
              </Button>
            </div>
          </nav>
        </div>
      )}
    </>
  );
}
