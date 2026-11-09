import { useEffect, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ChevronLeft, ChevronRight, Search, Sparkles, Trophy, type LucideIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
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

const CATEGORY_ICON: Record<string, LucideIcon> = { COOL_IDEA: Sparkles, OUTSTANDING_IMPACT: Trophy };

interface Props {
  /** Admin sees every Idea and can filter by Leader; a User sees the own ones (the server decides, `can()` shows). */
  admin: boolean;
  /** Admin row actions (F6-01), built by the page from `can()`; null hides the menu for that row. */
  rowActions?: (idea: IdeaSummary) => React.ReactNode;
}

/** Badge text, never colour alone: Leader filled, Member outline (design `VpC8a`). */
function RoleBadge({ relation }: { relation: 'LEADER' | 'MEMBER' }) {
  const { t } = useTranslation();
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        relation === 'LEADER' ? 'border-transparent bg-success text-success-foreground' : 'text-foreground',
      )}
    >
      {t(`pages.ideaList.relation.${relation}`)}
    </span>
  );
}

function CategoryLabel({ category }: { category: string }) {
  const { t } = useTranslation();
  const Icon = CATEGORY_ICON[category];
  return (
    <span className="inline-flex items-center gap-1.5">
      {Icon && <Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />}
      {t(`enums.category.${category}`, { defaultValue: category })}
    </span>
  );
}

export function IdeaList({ admin, rowActions }: Props) {
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

  const fmt = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' });
  const filtered = hasFilters(params);
  const clear = () => {
    setText('');
    setSp(toSearchParams({ ...params, page: 1, q: '', category: '', org: '', direction: '', leaderEmpNo: '', from: '', to: '' }), { replace: true });
  };
  const options = (name: 'category' | 'org' | 'direction', list: readonly string[]) => list.map((v) => ({ value: v, label: t(`enums.${name}.${v}`) }));
  const caption = admin ? t('nav.allIdeas') : t('nav.myIdeas');
  const team = (i: IdeaSummary) => `${i.memberCount + 1}/${MAX_TEAM}`; // the Leader has no member row
  const created = (i: IdeaSummary) => <time dateTime={i.createdAt}>{fmt.format(new Date(i.createdAt))}</time>;
  const actionsColumn = admin && rowActions !== undefined;

  const th = (label: React.ReactNode, className?: string, sortKey?: ListParams['sort']) => (
    <th
      scope="col"
      aria-sort={sortKey && params.sort === sortKey ? (params.order === 'asc' ? 'ascending' : 'descending') : undefined}
      className={cn('h-10 px-3 text-left align-middle font-medium whitespace-nowrap text-muted-foreground', className)}
    >
      {label}
    </th>
  );
  const td = 'px-3 py-3 align-middle';
  const narrowHidden = 'hidden lg:table-cell'; // 640–1023 px: no ESG direction and Created columns

  const from = data && data.total > 0 ? (data.page - 1) * data.size + 1 : 0;
  const to = data ? Math.min(data.total, (data.page - 1) * data.size + data.items.length) : 0;

  return (
    <>
      <form role="search" aria-label={t('pages.ideaList.filters')} onSubmit={(e) => e.preventDefault()} className="mb-4 flex flex-wrap items-center gap-2">
        <div className="relative w-full sm:w-64">
          <label htmlFor="q" className="sr-only">
            {t('pages.ideaList.search')}
          </label>
          <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input id="q" name="q" type="search" className="h-8 pl-8" placeholder={t('pages.ideaList.searchPlaceholder')} value={text} onChange={(e) => setText(e.target.value)} />
        </div>
        <FacetFilter label={t('fields.org')} value={params.org} options={options('org', ORGS)} onChange={(org) => update({ org })} />
        <FacetFilter label={t('fields.category')} value={params.category} options={options('category', CATEGORIES)} onChange={(category) => update({ category })} />
        <FacetFilter label={t('fields.direction')} value={params.direction} options={options('direction', DIRECTIONS)} onChange={(direction) => update({ direction })} />
        <DateRangeFilter label={t('pages.ideaList.created')} from={params.from} to={params.to} onChange={(patch) => update(patch)} />
        {admin && (
          <div className="w-36">
            <label htmlFor="leaderEmpNo" className="sr-only">
              {t('fields.leaderEmpNo')}
            </label>
            <Input
              id="leaderEmpNo"
              name="leaderEmpNo"
              className="h-8"
              inputMode="numeric"
              maxLength={6}
              placeholder={t('fields.leaderEmpNo')}
              value={params.leaderEmpNo}
              onChange={(e) => update({ leaderEmpNo: e.target.value.replace(/\D/g, '') })}
            />
          </div>
        )}
        {filtered && (
          <Button type="button" variant="ghost" size="sm" onClick={clear}>
            {t('pages.ideaList.clear')}
          </Button>
        )}
        <div className="ml-auto">
          <SortMenu sort={params.sort} order={params.order} onChange={(v) => update(v)} />
        </div>
      </form>

      <div aria-live="polite" role="status" className="sr-only">
        {data && t('pages.ideaList.total', { count: data.total })}
      </div>

      {isPending && <p role="status">{t('common.loading')}</p>}
      {isError && (
        <div role="alert" className="rounded-xl border border-destructive p-4">
          <p>{t(`errors.${error instanceof ApiError && error.code !== 'UNKNOWN' ? error.code : 'UNKNOWN'}`)}</p>
          {error instanceof ApiError && error.requestId && <p className="text-sm">{t('common.requestId', { id: error.requestId })}</p>}
          <Button type="button" variant="outline" className="mt-2" onClick={() => void refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      )}

      {data && data.items.length === 0 && (
        <p className="mt-4 rounded-xl border bg-card p-8 text-center text-muted-foreground">{filtered ? t('pages.ideaList.noMatches') : t('pages.ideaList.empty')}</p>
      )}

      {data && data.items.length > 0 && (
        <div className={isPlaceholderData ? 'opacity-60' : undefined} aria-busy={isPlaceholderData}>
          {wide ? (
            <div className="overflow-x-auto rounded-xl border bg-card">
              <table className="w-full text-sm">
                <caption className="sr-only">{caption}</caption>
                <thead className="border-b">
                  <tr>
                    {th(t('fields.ideaName'), undefined, 'name')}
                    {admin ? th(t('fields.leader')) : th(t('pages.ideaList.role'))}
                    {th(t('fields.category'))}
                    {th(t('fields.org'))}
                    {th(t('fields.direction'), narrowHidden)}
                    {th(t('pages.ideaList.team'))}
                    {th(t('pages.ideaList.created'), narrowHidden, 'createdAt')}
                    {actionsColumn && th(<span className="sr-only">{t('pages.ideaList.actions')}</span>, 'w-12')}
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((i) => (
                    <tr key={i.id} className="border-b last:border-0 hover:bg-muted/50">
                      <th scope="row" className={cn(td, 'text-left font-medium')}>
                        <Link to={`/ideas/${i.id}`} className="underline-offset-4 hover:underline focus-visible:underline">
                          {i.name}
                        </Link>
                      </th>
                      {admin ? (
                        <td className={td}>
                          {i.leader.name} <span className="text-muted-foreground">({i.leader.empNo})</span>
                        </td>
                      ) : (
                        <td className={td}>{i.myRelation && <RoleBadge relation={i.myRelation} />}</td>
                      )}
                      <td className={td}>
                        <CategoryLabel category={i.category} />
                      </td>
                      <td className={td}>{t(`enums.org.${i.org}`, { defaultValue: i.org })}</td>
                      <td className={cn(td, narrowHidden)}>{t(`enums.direction.${i.direction}`, { defaultValue: i.direction })}</td>
                      <td className={cn(td, 'tabular-nums')}>{team(i)}</td>
                      <td className={cn(td, narrowHidden, 'whitespace-nowrap text-muted-foreground')}>{created(i)}</td>
                      {actionsColumn && <td className={cn(td, 'text-right')}>{rowActions(i)}</td>}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <ul aria-label={caption} className="flex flex-col gap-3">
              {data.items.map((i) => (
                <li key={i.id} className="rounded-xl border bg-card p-4 text-sm">
                  <div className="flex items-start justify-between gap-2">
                    <Link to={`/ideas/${i.id}`} className="font-medium break-words underline-offset-4 hover:underline">
                      {i.name}
                    </Link>
                    {actionsColumn && rowActions(i)}
                  </div>
                  <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-2 text-muted-foreground">
                    {admin ? (
                      <span>
                        {t('fields.leader')}: {i.leader.name} ({i.leader.empNo})
                      </span>
                    ) : (
                      i.myRelation && <RoleBadge relation={i.myRelation} />
                    )}
                    <CategoryLabel category={i.category} />
                    <span className="tabular-nums">
                      {t('pages.ideaList.team')} {team(i)}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}

          <nav aria-label={t('pages.ideaList.paging')} className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-muted-foreground">
            <span>{t('pages.ideaList.showing', { from, to, total: data.total })}</span>
            <label className="flex items-center gap-2 sm:ml-auto">
              {t('pages.ideaList.perPage')}
              <span className="w-20">
                <NativeSelect value={params.size} onChange={(e) => update({ size: Number(e.target.value) })} className="h-8 text-foreground">
                  {SIZES.map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
                </NativeSelect>
              </span>
            </label>
            <span className="text-foreground">{t('pages.ideaList.pageOf', { page: data.page, pages })}</span>
            <div className="flex gap-1">
              <Button type="button" variant="outline" size="icon-sm" aria-label={t('pages.ideaList.prev')} disabled={params.page <= 1} onClick={() => update({ page: params.page - 1 }, { push: true })}>
                <ChevronLeft aria-hidden="true" />
              </Button>
              <Button type="button" variant="outline" size="icon-sm" aria-label={t('pages.ideaList.next')} disabled={params.page >= pages} onClick={() => update({ page: params.page + 1 }, { push: true })}>
                <ChevronRight aria-hidden="true" />
              </Button>
            </div>
          </nav>
        </div>
      )}
    </>
  );
}
