import { apiFetch, type components } from '@/shared/api';
import { CATEGORIES } from '@/shared/slides';
import { DIRECTIONS, ORGS } from './create';

export type IdeaPage = components['schemas']['IdeaPage'];
export type IdeaSummary = components['schemas']['IdeaSummary'];

export const SORTS = ['createdAt', 'name'] as const;
export type Sort = (typeof SORTS)[number];
export type Order = 'asc' | 'desc';
export const SIZES = [10, 20, 50, 100] as const;
export const DEFAULT_SIZE = 20;

/** The list state. Everything lives in the URL (Frontend.md §6.2), so a refresh or a shared link restores it. */
export interface ListParams {
  page: number;
  size: number;
  q: string;
  category: string;
  org: string;
  direction: string;
  leaderEmpNo: string;
  /** Local dates `YYYY-MM-DD`, both inclusive in the UI; sent as `[from, to)` instants. */
  from: string;
  to: string;
  sort: Sort;
  order: Order;
}

export const defaultOrder = (sort: Sort): Order => (sort === 'name' ? 'asc' : 'desc');

const oneOf = <T extends string>(list: readonly T[], v: string | null): T | '' => (list.find((x) => x === v) ?? '');
const posInt = (v: string | null, fallback: number, max = Number.MAX_SAFE_INTEGER) => {
  const n = v !== null && /^\d{1,9}$/.test(v) ? Number(v) : NaN;
  return n >= 1 ? Math.min(n, max) : fallback;
};
const ymd = (v: string) => v.split('-').map(Number) as [number, number, number];
const day = (v: string | null) => {
  if (v === null || !/^\d{4}-\d{2}-\d{2}$/.test(v)) return '';
  const [y, m, d] = ymd(v);
  const date = new Date(y, m - 1, d);
  return date.getMonth() === m - 1 && date.getDate() === d ? v : '';
};

/** Lenient: a hand-edited URL with a bad value falls back to the default instead of a 422 page. */
export function parseListParams(sp: URLSearchParams): ListParams {
  const sort = oneOf(SORTS, sp.get('sort')) || 'createdAt';
  return {
    page: posInt(sp.get('page'), 1),
    size: posInt(sp.get('size'), DEFAULT_SIZE, 100),
    q: sp.get('q') ?? '',
    category: oneOf(CATEGORIES, sp.get('category')),
    org: oneOf(ORGS, sp.get('org')),
    direction: oneOf(DIRECTIONS, sp.get('direction')),
    leaderEmpNo: /^\d{1,6}$/.test(sp.get('leaderEmpNo') ?? '') ? (sp.get('leaderEmpNo') as string) : '',
    from: day(sp.get('from')),
    to: day(sp.get('to')),
    sort,
    order: oneOf(['asc', 'desc'] as const, sp.get('order')) || defaultOrder(sort),
  };
}

/** URL form: defaults are left out, so the plain list has a plain `/ideas`. */
export function toSearchParams(p: ListParams): URLSearchParams {
  const sp = new URLSearchParams();
  if (p.page > 1) sp.set('page', String(p.page));
  if (p.size !== DEFAULT_SIZE) sp.set('size', String(p.size));
  if (p.q !== '') sp.set('q', p.q);
  for (const k of ['category', 'org', 'direction', 'leaderEmpNo', 'from', 'to'] as const) if (p[k] !== '') sp.set(k, p[k]);
  if (p.sort !== 'createdAt') sp.set('sort', p.sort);
  if (p.order !== defaultOrder(p.sort)) sp.set('order', p.order);
  return sp;
}

const localMidnight = (d: string, plusDays = 0) => {
  const [y, m, dd] = ymd(d);
  return new Date(y, m - 1, dd + plusDays).toISOString();
};

/** API form: dates become instants, `to` exclusive (the day after), so the whole last day is included. */
export function toApiQuery(p: ListParams): URLSearchParams {
  const sp = new URLSearchParams({ page: String(p.page), size: String(p.size), sort: p.sort, order: p.order });
  if (p.q.trim() !== '') sp.set('q', p.q.trim());
  for (const k of ['category', 'org', 'direction', 'leaderEmpNo'] as const) if (p[k] !== '') sp.set(k, p[k]);
  if (p.from !== '') sp.set('createdFrom', localMidnight(p.from));
  if (p.to !== '') sp.set('createdTo', localMidnight(p.to, 1));
  return sp;
}

/** Any filter (not paging or sorting) is set; tells "no Ideas yet" from "no results" (Frontend.md §6.8). */
export const hasFilters = (p: ListParams) => p.q.trim() !== '' || [p.category, p.org, p.direction, p.leaderEmpNo, p.from, p.to].some((v) => v !== '');

export const ideasKey = (p: ListParams) => ['ideas', p] as const;

export async function fetchIdeaPage(p: ListParams, signal?: AbortSignal): Promise<IdeaPage> {
  const res = await apiFetch(`/ideas?${toApiQuery(p)}`, { signal });
  return (await res.json()) as IdeaPage;
}
