import { QueryClient } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter } from 'react-router';
import { axe } from 'vitest-axe';
import { problem } from '@/shared/api/mocks/problem';
import { server } from '@/shared/api/mocks/server';
import { initI18n } from '@/shared/i18n';
import { App } from './App';
import { safeNext } from './access';
import { routes } from './router';
import { ROUTES, navFor, type Role } from './routes';

const API = 'http://localhost:3000/api/v1';
const me = (role: Role | null) =>
  http.get(`${API}/auth/me`, () =>
    role
      ? HttpResponse.json({ empNo: '1', name: 'Wang', email: role === 'ADMIN' ? null : 'w@x.io', role })
      : problem(401, 'UNAUTHENTICATED'),
  );
const deadline = (state: 'NOT_OPEN' | 'OPEN' | 'CLOSED', d: string | null = '2026-12-01T10:00:00.000Z') =>
  http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: d, state, serverTime: '2026-11-08T00:00:00.000Z' }));

async function open(path: string, role: Role | null, state: 'NOT_OPEN' | 'OPEN' | 'CLOSED' = 'OPEN') {
  server.use(me(role), deadline(state));
  await initI18n('en');
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const view = render(<App router={router} client={new QueryClient()} />);
  return { router, ...view };
}

/** A concrete URL for a route pattern. */
const concrete = (p: string) => p.replace(':id', '11111111-1111-4111-8111-111111111111');

describe('deadline banner (3 states)', () => {
  it.each([
    ['NOT_OPEN', 'Submissions are not open yet.'],
    ['OPEN', 'Submissions are open.'],
    ['CLOSED', 'Submissions are closed.'],
  ] as const)('%s', async (state, text) => {
    await open('/login', null, state);
    const region = await screen.findByRole('region', { name: 'Submission status' });
    expect(await within(region).findByText(new RegExp(text))).toBeInTheDocument();
    expect(region).toHaveAttribute('data-state', state);
  });

  it('shows an error state when the deadline cannot be loaded', async () => {
    server.use(me(null), http.get(`${API}/deadline`, () => problem(503, 'SERVICE_UNAVAILABLE')));
    await initI18n('en');
    render(<App router={createMemoryRouter(routes, { initialEntries: ['/login'] })} client={new QueryClient()} />);
    expect(await screen.findByText('The submission status could not be loaded.')).toBeInTheDocument();
  });

  it('is present in AppShell too', async () => {
    await open('/ideas', 'USER', 'CLOSED');
    expect(await screen.findByRole('region', { name: 'Submission status' })).toBeInTheDocument();
  });
});

describe('route access (Frontend.md §6.3)', () => {
  const nonPublic = ROUTES.filter((r) => r.access !== 'public');

  it.each(nonPublic.map((r) => [r.path] as const))('visitor on %s → /login?next=', async (path) => {
    const { router } = await open(concrete(path), null);
    await screen.findByRole('heading', { level: 1 });
    expect(router.state.location.pathname).toBe('/login');
    expect(router.state.location.search).toBe(`?next=${encodeURIComponent(concrete(path))}`);
  });

  it.each(ROUTES.filter((r) => r.access === 'public').map((r) => [r.path] as const))('logged-in user on %s → /ideas', async (path) => {
    const { router } = await open(path, 'USER');
    await screen.findByRole('heading', { level: 1 });
    expect(router.state.location.pathname).toBe('/ideas');
  });

  const mismatches = ROUTES.flatMap((r) =>
    (['USER', 'ADMIN'] as const).filter((role) => r.access === 'USER' || r.access === 'ADMIN' ? r.access !== role : false).map((role) => [role, r.path] as const),
  );
  it.each(mismatches)('%s on %s gets the same 404 as an unknown route', async (role, path) => {
    await open(concrete(path), role);
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
    expect(screen.getByText('The page or item you asked for was not found.')).toBeInTheDocument();
  });

  it.each(['USER', 'ADMIN'] as const)('%s on an unknown route gets the 404 page', async (role) => {
    await open('/nope', role);
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
  });

  it('allowed routes render their page', async () => {
    await open('/admin/deadline', 'ADMIN');
    expect(await screen.findByRole('heading', { level: 1, name: 'Deadline' })).toBeInTheDocument();
  });

  it('/ redirects by identity', async () => {
    const { router } = await open('/', null);
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
  });
});

describe('navigation matches the route table', () => {
  it.each([
    ['USER', ['Ideas', 'Join requests']],
    ['ADMIN', ['Ideas', 'Deadline', 'Statistics']],
  ] as const)('%s', async (role, labels) => {
    await open('/ideas', role);
    const nav = await screen.findByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getAllByRole('link').map((a) => a.textContent)).toEqual(labels);
    expect(within(nav).getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual(navFor(role).map((r) => r.path));
    expect(!!screen.queryByText('Administrator')).toBe(role === 'ADMIN');
  });
});

describe('safeNext (§6.4)', () => {
  it.each([
    ['/ideas?page=2', 'USER', '/ideas?page=2'],
    ['/admin/deadline', 'USER', '/ideas'],
    ['/admin/deadline', 'ADMIN', '/admin/deadline'],
    ['//evil.com', 'USER', '/ideas'],
    ['/\\evil.com', 'USER', '/ideas'],
    ['https://evil.com', 'USER', '/ideas'],
    ['/login', 'USER', '/ideas'],
    ['/unknown', 'USER', '/ideas'],
    [null, 'USER', '/ideas'],
  ] as const)('%s as %s → %s', (next, role, out) => {
    expect(safeNext(next, role)).toBe(out);
  });
});

describe('accessibility', () => {
  it.each([
    ['/login', null],
    ['/ideas', 'USER'],
    ['/admin/deadline', 'ADMIN'],
    ['/nope', 'USER'],
  ] as const)('%s as %s has no axe violations', async (path, role) => {
    const { container } = await open(path, role);
    await screen.findByRole('heading', { level: 1 });
    await screen.findByRole('region', { name: 'Submission status' });
    expect((await axe(container)).violations).toEqual([]);
  });
});
