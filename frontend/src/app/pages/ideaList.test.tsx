import { QueryClient } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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

const HEADING = { en: { USER: 'My Ideas', ADMIN: 'All Ideas' }, 'zh-TW': { USER: '我的提案', ADMIN: '所有提案' } } as const;

async function open(url = '/ideas', { me = USER as typeof USER | typeof ADMIN, locale = 'en' as Locale } = {}) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state: 'OPEN', serverTime: '2026-11-08T00:00:00.000Z' })),
  );
  await initI18n(locale);
  const router = createMemoryRouter(routes, { initialEntries: [url] });
  const view = render(<App router={router} client={new QueryClient()} />);
  await screen.findByRole('heading', { level: 1, name: HEADING[locale][me.role as 'USER' | 'ADMIN'] });
  return { router, user: userEvent.setup(), ...view };
}

const search = (router: ReturnType<typeof createMemoryRouter>) => new URLSearchParams(router.state.location.search);

// Radix Popover / DropdownMenu need these browser APIs, which jsdom lacks.
beforeAll(() => {
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.releasePointerCapture ??= () => {};
  Element.prototype.scrollIntoView ??= () => {};
});

/** `matchMedia` reporting a viewport below 640 px until the returned function restores it. */
function narrowViewport() {
  const original = window.matchMedia;
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
  return () => {
    window.matchMedia = original;
  };
}

const facet = (name: string) => screen.getByRole('button', { name: new RegExp(`^${name}`) });

describe('Idea list: content', () => {
  it('shows rows with a link to the detail page, the role badge, n/4 and the header actions for a User', async () => {
    listReturns(() => page([idea(1), idea(2, { myRelation: 'MEMBER', name: 'Second', memberCount: 2 })]));
    await open();
    const table = await screen.findByRole('table', { name: 'My Ideas' });
    expect(within(table).getByRole('link', { name: 'Idea 1' })).toHaveAttribute('href', '/ideas/0193a1b2-0000-7000-8000-000000000001');
    const first = within(table).getByRole('row', { name: /Idea 1/ });
    expect(within(first).getByText('Leader')).toBeInTheDocument();
    expect(within(first).getByText('2/4')).toBeInTheDocument(); // memberCount 1 + the Leader
    const row = within(table).getByRole('row', { name: /Second/ });
    expect(within(row).getByText('Member')).toBeInTheDocument();
    expect(within(row).getByText('3/4')).toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Your role' })).toBeInTheDocument();
    expect(within(table).queryByRole('columnheader', { name: 'Leader' })).not.toBeInTheDocument();
    expect(within(table).getAllByRole('link')).toHaveLength(2); // one tab stop per row
    const main = within(screen.getByRole('main'));
    expect(main.getByRole('link', { name: 'New Idea' })).toHaveAttribute('href', '/ideas/new');
    expect(main.getByRole('link', { name: 'Join a team' })).toHaveAttribute('href', '/join-requests');
    expect(screen.queryByRole('button', { name: /^Actions for/ })).not.toBeInTheDocument();
    expect(screen.getByText('Showing 1–2 of 2 Ideas')).toBeInTheDocument();
  });

  it('for an Admin: title "All Ideas", no header actions, a Leader column instead of the role, a Leader filter', async () => {
    listReturns(() => page([idea(1, { myRelation: null })]));
    await open('/ideas', { me: ADMIN });
    const table = await screen.findByRole('table', { name: 'All Ideas' });
    const main = within(screen.getByRole('main'));
    expect(main.queryByRole('link', { name: 'New Idea' })).not.toBeInTheDocument();
    expect(main.queryByRole('link', { name: 'Join a team' })).not.toBeInTheDocument();
    expect(within(table).queryByRole('columnheader', { name: 'Your role' })).not.toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Leader' })).toBeInTheDocument();
    expect(within(table).getByRole('row', { name: /Idea 1/ })).toHaveTextContent('王小明 (089290)');
    expect(screen.getByLabelText('Leader employee ID')).toBeInTheDocument();
  });

  it('Admin row actions: download, replace (via the detail) and delete with confirmation', async () => {
    let deleted = false;
    const seen = listReturns(() => page(deleted ? [] : [idea(1, { myRelation: null })]));
    server.use(
      http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
      http.delete(`${API}/ideas/:id`, () => {
        deleted = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = await open('/ideas', { me: ADMIN });
    await screen.findByRole('table');
    await user.click(screen.getByRole('button', { name: 'Actions for Idea 1' }));
    const menu = await screen.findByRole('menu');
    expect(within(menu).getByRole('menuitem', { name: 'Download' })).toBeInTheDocument();
    expect(within(menu).getByRole('menuitem', { name: 'Replace slides' })).toHaveAttribute('href', '/ideas/0193a1b2-0000-7000-8000-000000000001');
    await user.click(within(menu).getByRole('menuitem', { name: 'Delete' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Delete this Idea?' });
    const before = seen.length;
    await user.click(within(dialog).getByRole('button', { name: 'Delete Idea' }));
    await waitFor(() => expect(seen.length).toBeGreaterThan(before));
    expect(await screen.findByText('No Ideas yet.')).toBeInTheDocument();
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
    expect(within(screen.getByRole('main')).getByRole('link', { name: '新增提案' })).toBeInTheDocument();
  });

  it('below 640 px the table becomes one card per Idea with the same link and data', async () => {
    const restore = narrowViewport();
    try {
      listReturns(() => page([idea(1, { memberCount: 3 })]));
      await open();
      const list = await screen.findByRole('list', { name: 'My Ideas' });
      expect(screen.queryByRole('table')).not.toBeInTheDocument();
      const card = within(list).getByRole('listitem');
      expect(within(card).getByRole('link', { name: 'Idea 1' })).toHaveAttribute('href', '/ideas/0193a1b2-0000-7000-8000-000000000001');
      expect(card).toHaveTextContent('Leader');
      expect(card).toHaveTextContent('Cool Idea Award');
      expect(card).toHaveTextContent('4/4');
    } finally {
      restore();
    }
  });

  it.each(['en', 'zh-TW'] as const)('has no axe violations: filled, empty and no-match states (%s)', async (locale) => {
    listReturns(() => page([idea(1), idea(2, { myRelation: 'MEMBER' })]));
    const filled = await open('/ideas', { locale });
    await screen.findByRole('table');
    expect((await axe(filled.container)).violations).toEqual([]);
    filled.unmount();
    listReturns(() => page([]));
    const empty = await open('/ideas', { locale });
    await screen.findByText(locale === 'en' ? 'No Ideas yet.' : '目前沒有提案。');
    expect((await axe(empty.container)).violations).toEqual([]);
    empty.unmount();
    const noMatch = await open('/ideas?org=BSID', { locale });
    await screen.findByRole('button', { name: locale === 'en' ? 'Clear filters' : '清除篩選' });
    expect((await axe(noMatch.container)).violations).toEqual([]);
  });

  it('has no axe violations for an Admin with row actions and with an open filter popover', async () => {
    listReturns(() => page([idea(1, { myRelation: null })]));
    const { container, user } = await open('/ideas', { me: ADMIN });
    await screen.findByRole('table');
    expect((await axe(container)).violations).toEqual([]);
    await user.click(facet('Organization'));
    await screen.findByRole('radio', { name: 'BSID' });
    expect((await axe(document.body)).violations).toEqual([]);
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
    expect(facet('Organization')).toHaveTextContent('BSID');
    expect(facet('Category')).toHaveTextContent('Cool Idea Award');
    expect(facet('Direction')).toHaveTextContent('Talent Development');
    expect(facet('Created')).toHaveTextContent('2026');
    expect(screen.getByRole('button', { name: 'Sort: Name Z–A' })).toBeInTheDocument();
    expect(screen.getByLabelText('Search by name')).toHaveValue('supply');
    expect(screen.getByText('Page 3 of 4')).toBeInTheDocument();
    expect(screen.getByText('Showing 101–101 of 200 Ideas')).toBeInTheDocument();
  });

  it('ignores bad values in a hand-edited URL instead of calling the API with them', async () => {
    const seen = listReturns(() => page([idea(1)]));
    await open('/ideas?page=-2&size=1000&org=NOPE&sort=secret&order=up&from=2026-13-40');
    await screen.findByRole('table');
    expect(Object.fromEntries(seen.at(-1)!)).toEqual({ page: '1', size: '100', sort: 'createdAt', order: 'desc' });
  });

  it.each([
    ['Organization', 'org', 'ICSD', 'ICSD'],
    ['Category', 'category', 'Outstanding Impact Award', 'OUTSTANDING_IMPACT'],
    ['Direction', 'direction', 'Green Manufacturing', 'GREEN_MANUFACTURING'],
  ])('the %s filter writes its parameter to the URL and the request (page 1), and clears it again', async (label, param, option, value) => {
    const seen = listReturns(() => page([idea(1)], { total: 100 }));
    const { router, user } = await open('/ideas?page=2');
    await screen.findByRole('table');
    await user.click(facet(label));
    await user.click(await screen.findByRole('radio', { name: option }));
    await waitFor(() => expect(search(router).toString()).toBe(`${param}=${value}`));
    await waitFor(() => expect(seen.at(-1)!.get(param)).toBe(value));
    expect(screen.queryByRole('radio', { name: option })).not.toBeInTheDocument(); // single choice closes the popover
    await user.click(screen.getByRole('button', { name: `Clear the ${label} filter` }));
    await waitFor(() => expect(router.state.location.search).toBe(''));
    await waitFor(() => expect(seen.at(-1)!.has(param)).toBe(false));
  });

  it('the "All" option in a filter clears it too', async () => {
    listReturns(() => page([idea(1)]));
    const { router, user } = await open('/ideas?org=BSID');
    await screen.findByRole('table');
    await user.click(facet('Organization'));
    expect(await screen.findByRole('radio', { name: 'BSID' })).toBeChecked();
    await user.click(screen.getByRole('radio', { name: 'All' }));
    await waitFor(() => expect(router.state.location.search).toBe(''));
  });

  it('the Created filter maps from / to to createdFrom / createdTo and clears both', async () => {
    const seen = listReturns(() => page([idea(1)]));
    const { router, user } = await open();
    await screen.findByRole('table');
    await user.click(facet('Created'));
    fireEvent.change(await screen.findByLabelText('Created from'), { target: { value: '2026-11-01' } });
    await waitFor(() => expect(search(router).get('from')).toBe('2026-11-01'));
    fireEvent.change(screen.getByLabelText('Created until'), { target: { value: '2026-11-03' } });
    await waitFor(() => expect(search(router).toString()).toBe('from=2026-11-01&to=2026-11-03'));
    await waitFor(() => expect(seen.at(-1)!.get('createdTo')).toBe(new Date(2026, 10, 4).toISOString()));
    expect(seen.at(-1)!.get('createdFrom')).toBe(new Date(2026, 10, 1).toISOString());
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: 'Clear the Created filter' }));
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
    expect(facet('Organization')).toHaveTextContent('TSID');
    expect(seen.at(-1)!.get('sort')).toBe('name');
    expect(seen.at(-1)!.get('order')).toBe('asc');
  });

  it('the sort menu writes sort and order; aria-sort follows on the column', async () => {
    const seen = listReturns(() => page([idea(1)]));
    const { router, user } = await open();
    await screen.findByRole('table');
    expect(screen.getByRole('columnheader', { name: /Created/ })).toHaveAttribute('aria-sort', 'descending');
    const choose = async (current: string, next: string) => {
      await user.click(screen.getByRole('button', { name: `Sort: ${current}` }));
      await user.click(await screen.findByRole('menuitemradio', { name: next }));
    };
    await choose('Newest first', 'Name A–Z');
    await waitFor(() => expect(search(router).toString()).toBe('sort=name'));
    expect(screen.getByRole('columnheader', { name: /Idea name/ })).toHaveAttribute('aria-sort', 'ascending');
    await choose('Name A–Z', 'Name Z–A');
    await waitFor(() => expect(search(router).toString()).toBe('sort=name&order=desc'));
    await waitFor(() => expect(seen.at(-1)!.get('order')).toBe('desc'));
    await choose('Name Z–A', 'Oldest first');
    await waitFor(() => expect(search(router).toString()).toBe('order=asc'));
    await waitFor(() => expect(Object.fromEntries(seen.at(-1)!)).toMatchObject({ sort: 'createdAt', order: 'asc' }));
    await choose('Oldest first', 'Newest first');
    await waitFor(() => expect(router.state.location.search).toBe(''));
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
