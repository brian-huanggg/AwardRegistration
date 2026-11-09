import { QueryClient } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter } from 'react-router';
import { axe } from 'vitest-axe';
import { clearCsrfToken } from '@/shared/api';
import { problem } from '@/shared/api/mocks/problem';
import { server } from '@/shared/api/mocks/server';
import { initI18n, type Locale } from '@/shared/i18n';
import { App } from '../App';
import { routes } from '../router';

const API = 'http://localhost:3000/api/v1';
const USER = { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' };
const ADMIN = { empNo: '000001', name: 'Admin', email: null, role: 'ADMIN' };

const idea = (n: number, over: Record<string, unknown> = {}) => ({
  id: `0193a1b2-0000-7000-8000-${String(n).padStart(12, '0')}`,
  name: `Idea ${n}`,
  category: 'COOL_IDEA',
  org: 'BSID',
  direction: 'TALENT_DEVELOPMENT',
  leader: { empNo: '089290', name: '王小明' },
  memberCount: 1,
  myRelation: 'LEADER',
  createdAt: '2026-11-02T03:00:00.000Z',
  slidesUpdatedAt: '2026-11-02T03:00:00.000Z',
  ...over,
});

/** Records every `GET /ideas` query and answers with `respond(query)`. */
function listReturns(respond: (q: URLSearchParams) => Response | Promise<Response>) {
  const seen: URLSearchParams[] = [];
  server.use(
    http.get(`${API}/ideas`, ({ request }) => {
      const q = new URL(request.url).searchParams;
      seen.push(q);
      return respond(q);
    }),
  );
  return seen;
}

const page = (items: unknown[], over: Record<string, unknown> = {}) =>
  HttpResponse.json({ items, page: 1, size: 20, total: items.length, ...over });

async function open(url = '/ideas', { me = USER as typeof USER | typeof ADMIN, locale = 'en' as Locale } = {}) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state: 'OPEN', serverTime: '2026-11-08T00:00:00.000Z' })),
  );
  await initI18n(locale);
  const router = createMemoryRouter(routes, { initialEntries: [url] });
  const view = render(<App router={router} client={new QueryClient()} />);
  await screen.findByRole('heading', { level: 1, name: locale === 'en' ? 'Ideas' : '提案' });
  return { router, user: userEvent.setup(), ...view };
}

const search = (router: ReturnType<typeof createMemoryRouter>) => new URLSearchParams(router.state.location.search);

describe('Idea list: content', () => {
  it('shows rows with a link to the detail page, the own role and the Create button for a User', async () => {
    listReturns(() => page([idea(1), idea(2, { myRelation: 'MEMBER', name: 'Second' })]));
    await open();
    const table = await screen.findByRole('table', { name: 'Ideas' });
    expect(within(table).getByRole('link', { name: 'Idea 1' })).toHaveAttribute('href', '/ideas/0193a1b2-0000-7000-8000-000000000001');
    const row = within(table).getByRole('row', { name: /Second/ });
    expect(within(row).getByText('Member')).toBeInTheDocument();
    expect(within(row).getByText('王小明 (089290)')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Create an Idea' })).toBeInTheDocument();
  });

  it('for an Admin: no Create button, no role column, a Leader filter', async () => {
    listReturns(() => page([idea(1, { myRelation: null })]));
    await open('/ideas', { me: ADMIN });
    await screen.findByRole('table');
    expect(screen.queryByRole('link', { name: 'Create an Idea' })).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Your role' })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Leader employee ID')).toBeInTheDocument();
  });

  it('tells "no Ideas yet" from "no results for these filters"', async () => {
    listReturns(() => page([]));
    await open();
    expect(await screen.findByText('No Ideas yet.')).toBeInTheDocument();
    await open('/ideas?org=BSID');
    expect(await screen.findByText('No Ideas match these filters.')).toBeInTheDocument();
  });

  it('shows the error with the request ID and retries', async () => {
    let fail = true;
    listReturns(() => (fail ? problem(500, 'INTERNAL_ERROR', { requestId: 'req-7' }) : page([idea(1)])));
    const { user } = await open();
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Request ID: req-7');
    fail = false;
    await user.click(within(alert).getByRole('button', { name: 'Try again' }));
    expect(await screen.findByRole('table')).toBeInTheDocument();
  });

  it('renders in zh-TW', async () => {
    listReturns(() => page([idea(1)]));
    await open('/ideas', { locale: 'zh-TW' });
    expect(await screen.findByRole('columnheader', { name: /名稱|提案名稱/ })).toBeInTheDocument();
    expect(screen.getByText('提案人')).toBeInTheDocument();
  });

  it.each(['en', 'zh-TW'] as const)('has no axe violations: list and empty state (%s)', async (locale) => {
    listReturns((q) => (q.get('org') ? page([]) : page([idea(1)])));
    const { container, unmount } = await open('/ideas', { locale });
    await screen.findByRole('table');
    expect((await axe(container)).violations).toEqual([]);
    unmount();
    const empty = await open('/ideas?org=BSID', { locale });
    await screen.findByRole('button', { name: locale === 'en' ? 'Clear filters' : '清除篩選' });
    expect((await axe(empty.container)).violations).toEqual([]);
  });
});

describe('Idea list: state lives in the URL', () => {
  it('reads filters, sort and paging from the URL and sends them (dates as [from, to) instants)', async () => {
    const seen = listReturns(() => page([idea(1)], { page: 3, size: 50, total: 200 }));
    await open('/ideas?page=3&size=50&org=BSID&category=COOL_IDEA&direction=TALENT_DEVELOPMENT&q=supply&sort=name&order=desc&from=2026-11-01&to=2026-11-03');
    await screen.findByRole('table');
    const q = seen.at(-1)!;
    expect(Object.fromEntries(q)).toEqual({
      page: '3',
      size: '50',
      org: 'BSID',
      category: 'COOL_IDEA',
      direction: 'TALENT_DEVELOPMENT',
      q: 'supply',
      sort: 'name',
      order: 'desc',
      createdFrom: new Date(2026, 10, 1).toISOString(),
      createdTo: new Date(2026, 10, 4).toISOString(),
    });
    expect(screen.getByLabelText('Organization')).toHaveValue('BSID');
    expect(screen.getByLabelText('Search by name')).toHaveValue('supply');
    expect(screen.getByText('Page 3 of 4')).toBeInTheDocument();
  });

  it('ignores bad values in a hand-edited URL instead of calling the API with them', async () => {
    const seen = listReturns(() => page([idea(1)]));
    await open('/ideas?page=-2&size=1000&org=NOPE&sort=secret&order=up&from=2026-13-40');
    await screen.findByRole('table');
    expect(Object.fromEntries(seen.at(-1)!)).toEqual({ page: '1', size: '100', sort: 'createdAt', order: 'desc' });
  });

  it('a filter change goes to the URL and back to page 1; defaults stay out of the URL', async () => {
    listReturns(() => page([idea(1)], { total: 100 }));
    const { router, user } = await open('/ideas?page=2');
    await screen.findByRole('table');
    await user.selectOptions(screen.getByLabelText('Organization'), 'ICSD');
    await waitFor(() => expect(search(router).toString()).toBe('org=ICSD'));
    await user.selectOptions(screen.getByLabelText('Organization'), 'All');
    await waitFor(() => expect(router.state.location.search).toBe(''));
  });

  it('debounces the search box into the URL', async () => {
    const seen = listReturns(() => page([idea(1)]));
    const { router, user } = await open();
    await screen.findByRole('table');
    const before = seen.length;
    await user.type(screen.getByLabelText('Search by name'), 'abc');
    expect(seen.length).toBe(before); // nothing sent per keystroke
    await waitFor(() => expect(search(router).get('q')).toBe('abc'));
    await waitFor(() => expect(seen.at(-1)!.get('q')).toBe('abc'));
    expect(seen.length).toBe(before + 1);
  });

  it('survives a refresh: remounting on the same URL restores the controls and the request', async () => {
    const seen = listReturns(() => page([idea(1)]));
    const first = await open('/ideas?org=TSID&sort=name');
    await screen.findByRole('table');
    const url = first.router.state.location.pathname + first.router.state.location.search;
    first.unmount();
    await open(url);
    await screen.findByRole('table');
    expect(screen.getByLabelText('Organization')).toHaveValue('TSID');
    expect(seen.at(-1)!.get('sort')).toBe('name');
    expect(seen.at(-1)!.get('order')).toBe('asc');
  });

  it('sorting: a click toggles the order, a new column starts at its default; aria-sort follows', async () => {
    const seen = listReturns(() => page([idea(1)]));
    const { router, user } = await open();
    await screen.findByRole('table');
    expect(screen.getByRole('columnheader', { name: /Created/ })).toHaveAttribute('aria-sort', 'descending');
    await user.click(screen.getByRole('button', { name: 'Idea name' }));
    await waitFor(() => expect(search(router).toString()).toBe('sort=name'));
    expect(screen.getByRole('columnheader', { name: /Idea name/ })).toHaveAttribute('aria-sort', 'ascending');
    await user.click(screen.getByRole('button', { name: /Idea name/ }));
    await waitFor(() => expect(search(router).toString()).toBe('sort=name&order=desc'));
    await waitFor(() => expect(seen.at(-1)!.get('order')).toBe('desc'));
  });

  it('paging pushes history entries; the size select resets to page 1', async () => {
    listReturns((q) => page([idea(1)], { page: Number(q.get('page')), size: Number(q.get('size')), total: 100 }));
    const { router, user } = await open();
    await screen.findByText('Page 1 of 5');
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText('Page 2 of 5');
    expect(search(router).get('page')).toBe('2');
    await router.navigate(-1);
    await screen.findByText('Page 1 of 5');
    await user.click(screen.getByRole('button', { name: 'Next' }));
    await screen.findByText('Page 2 of 5');
    await user.selectOptions(screen.getByLabelText('Per page'), '50');
    await screen.findByText('Page 1 of 2');
    expect(search(router).toString()).toBe('size=50');
  });

  it('a page beyond the last one moves to the last page', async () => {
    listReturns((q) => (q.get('page') === '9' ? page([], { page: 9, total: 25 }) : page([idea(1)], { page: Number(q.get('page')), total: 25 })));
    const { router } = await open('/ideas?page=9');
    await waitFor(() => expect(search(router).get('page')).toBe('2'));
  });

  it('clear filters keeps sort and size, empties the rest', async () => {
    listReturns(() => page([idea(1)]));
    const { router, user } = await open('/ideas?org=BSID&q=x&sort=name&size=50');
    await screen.findByRole('table');
    await user.click(screen.getByRole('button', { name: 'Clear filters' }));
    await waitFor(() => expect(search(router).toString()).toBe('size=50&sort=name'));
    expect(screen.getByLabelText('Search by name')).toHaveValue('');
  });

  it('the Admin Leader filter sends leaderEmpNo (digits only)', async () => {
    const seen = listReturns(() => page([idea(1, { myRelation: null })]));
    const { router, user } = await open('/ideas', { me: ADMIN });
    await screen.findByRole('table');
    await user.type(screen.getByLabelText('Leader employee ID'), '08a929');
    await waitFor(() => expect(search(router).get('leaderEmpNo')).toBe('08929'));
    await waitFor(() => expect(seen.at(-1)!.get('leaderEmpNo')).toBe('08929'));
  });
});
