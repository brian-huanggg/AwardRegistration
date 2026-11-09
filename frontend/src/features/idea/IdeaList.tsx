import { useEffect, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { ApiError } from '@/shared/api';
import { Button } from '@/shared/ui/button';
import { Field, FieldLabel } from '@/shared/ui/field';
import { Input } from '@/shared/ui/input';
import { NativeSelect } from '@/shared/ui/native-select';
import { CATEGORIES, DIRECTIONS, ORGS } from './create';
import {
  defaultOrder,
  fetchIdeaPage,
  hasFilters,
  ideasKey,
  parseListParams,
  SIZES,
  toSearchParams,
  type ListParams,
  type Sort,
} from './list';

const DEBOUNCE_MS = 300;

interface Props {
  /** Admin sees every Idea and can filter by Leader; a User sees the own ones (the server decides, `can()` shows). */
  admin: boolean;
  /** Extra controls next to the filters, e.g. the "Create" button for a User. */
  actions?: React.ReactNode;
}

export function IdeaList({ admin, actions }: Props) {
  const { t, i18n } = useTranslation();
  const [sp, setSp] = useSearchParams();
  const params = parseListParams(sp);

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

  const sortHeader = (sort: Sort, label: string) => {
    const active = params.sort === sort;
    const next = active ? (params.order === 'asc' ? 'desc' : 'asc') : defaultOrder(sort);
    return (
      <th key={sort} scope="col" aria-sort={active ? (params.order === 'asc' ? 'ascending' : 'descending') : undefined} className="border-b p-3 font-medium text-muted-foreground">
        <button type="button" className="font-medium underline-offset-4 hover:underline" onClick={() => update({ sort, order: next })}>
          {label}
          {active && <span aria-hidden="true">{params.order === 'asc' ? ' ↑' : ' ↓'}</span>}
        </button>
      </th>
    );
  };
  const plainHeader = (label: string) => (
    <th key={label} scope="col" className="border-b p-3 font-medium text-muted-foreground">
      {label}
    </th>
  );

  const box = (id: string, label: string, control: React.ReactNode) => (
    <Field key={id} className="w-auto min-w-40">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {control}
    </Field>
  );

  const select = (name: 'category' | 'org' | 'direction', list: readonly string[]) => (
    box(
      name,
      t(`fields.${name}`),
      <NativeSelect id={name} name={name} value={params[name]} onChange={(e) => update({ [name]: e.target.value })}>
        <option value="">{t('pages.ideaList.all')}</option>
        {list.map((v) => (
          <option key={v} value={v}>
            {t(`enums.${name}.${v}`)}
          </option>
        ))}
      </NativeSelect>,
    )
  );

  return (
    <>
      <form role="search" aria-label={t('pages.ideaList.filters')} onSubmit={(e) => e.preventDefault()} className="mb-4 flex flex-wrap items-end gap-x-4">
        {box('q', t('pages.ideaList.search'), <Input id="q" name="q" type="search" value={text} onChange={(e) => setText(e.target.value)} />)}
        {select('category', CATEGORIES)}
        {select('org', ORGS)}
        {select('direction', DIRECTIONS)}
        {admin &&
          box(
            'leaderEmpNo',
            t('fields.leaderEmpNo'),
            <Input id="leaderEmpNo" name="leaderEmpNo" inputMode="numeric" maxLength={6} value={params.leaderEmpNo} onChange={(e) => update({ leaderEmpNo: e.target.value.replace(/\D/g, '') })} />,
          )}
        {box('from', t('pages.ideaList.from'), <Input id="from" name="from" type="date" value={params.from} max={params.to || undefined} onChange={(e) => update({ from: e.target.value })} />)}
        {box('to', t('pages.ideaList.to'), <Input id="to" name="to" type="date" value={params.to} min={params.from || undefined} onChange={(e) => update({ to: e.target.value })} />)}
        <div className="flex gap-2">
          {filtered && (
            <Button type="button" variant="outline" onClick={clear}>
              {t('pages.ideaList.clear')}
            </Button>
          )}
          {actions}
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
        <p className="mt-4">{filtered ? t('pages.ideaList.noMatches') : t('pages.ideaList.empty')}</p>
      )}

      {data && data.items.length > 0 && (
        <div className={isPlaceholderData ? 'opacity-60' : undefined} aria-busy={isPlaceholderData}>
          <div className="overflow-x-auto">
            <table className="w-full rounded-xl border bg-card text-left text-sm">
              <caption className="sr-only">{t('nav.ideas')}</caption>
              <thead>
                <tr>
                  {sortHeader('name', t('fields.ideaName'))}
                  {plainHeader(t('fields.category'))}
                  {plainHeader(t('fields.org'))}
                  {plainHeader(t('fields.direction'))}
                  {plainHeader(t('fields.leader'))}
                  {plainHeader(t('fields.members'))}
                  {!admin && plainHeader(t('pages.ideaList.role'))}
                  {sortHeader('createdAt', t('pages.ideaList.created'))}
                </tr>
              </thead>
              <tbody>
                {data.items.map((i) => (
                  <tr key={i.id} className="border-b last:border-0">
                    <th scope="row" className="p-3 font-medium">
                      <Link to={`/ideas/${i.id}`} className="text-primary underline underline-offset-4">
                        {i.name}
                      </Link>
                    </th>
                    <td className="p-3">{t(`enums.category.${i.category}`, { defaultValue: i.category })}</td>
                    <td className="p-3">{t(`enums.org.${i.org}`, { defaultValue: i.org })}</td>
                    <td className="p-3">{t(`enums.direction.${i.direction}`, { defaultValue: i.direction })}</td>
                    <td className="p-3">
                      {i.leader.name} ({i.leader.empNo})
                    </td>
                    <td className="p-3">{i.memberCount}</td>
                    {!admin && <td className="p-3">{i.myRelation ? t(`pages.ideaList.relation.${i.myRelation}`) : ''}</td>}
                    <td className="p-3">
                      <time dateTime={i.createdAt}>{fmt.format(new Date(i.createdAt))}</time>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <nav aria-label={t('pages.ideaList.paging')} className="mt-4 flex flex-wrap items-center gap-3 text-sm">
            <Button type="button" variant="outline" size="sm" disabled={params.page <= 1} onClick={() => update({ page: params.page - 1 }, { push: true })}>
              {t('pages.ideaList.prev')}
            </Button>
            <span>{t('pages.ideaList.pageOf', { page: data.page, pages })}</span>
            <Button type="button" variant="outline" size="sm" disabled={params.page >= pages} onClick={() => update({ page: params.page + 1 }, { push: true })}>
              {t('pages.ideaList.next')}
            </Button>
            <label className="ml-auto flex items-center gap-2">
              {t('pages.ideaList.perPage')}
              <NativeSelect value={params.size} onChange={(e) => update({ size: Number(e.target.value) })} className="w-20">
                {SIZES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </NativeSelect>
            </label>
          </nav>
        </div>
      )}
    </>
  );
}
