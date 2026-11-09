import { QueryClient } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { delay, http, HttpResponse } from 'msw';
import { createMemoryRouter } from 'react-router';
import { axe } from 'vitest-axe';
import { clearCsrfToken } from '@/shared/api';
import { problem } from '@/shared/api/mocks/problem';
import { server } from '@/shared/api/mocks/server';
import { initI18n } from '@/shared/i18n';
import { App } from './App';
import { routes } from './router';
import { navFor, type Role } from './routes';

const API = 'http://localhost:3000/api/v1';
const ID = '0193a1b2-0000-7000-8000-000000000001';
const ME = {
  USER: { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' },
  ADMIN: { empNo: '000001', name: '陳主任', email: null, role: 'ADMIN' },
} as const;

type DeadlineMock = 'NOT_OPEN' | 'OPEN' | 'CLOSED' | 'loading' | 'error';

async function open(path: string, role: Role = 'USER', deadline: DeadlineMock = 'OPEN') {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(ME[role])),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
    http.get(`${API}/deadline`, async () => {
      if (deadline === 'loading') await delay('infinite');
      if (deadline === 'error') return problem(500, 'INTERNAL_ERROR');
      return HttpResponse.json({ deadline: '2026-12-01T10:00:00.000Z', state: deadline, serverTime: '2026-11-08T00:00:00.000Z' });
    }),
    http.get(`${API}/ideas`, () => HttpResponse.json({ items: [], page: 1, size: 20, total: 0 })),
    http.get(`${API}/ideas/:id`, () => problem(404, 'NOT_FOUND')),
    http.get(`${API}/join-requests`, () => HttpResponse.json({ items: [] })),
  );
  await initI18n('en');
  const client = new QueryClient();
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const view = render(<App router={router} client={client} />);
  await screen.findByRole('heading', { level: 1 });
  return { router, client, user: userEvent.setup(), ...view };
}

const mainNav = () => screen.getByRole('navigation', { name: 'Main navigation' });

describe('sidebar navigation (Frontend.md §6.2, §6.5)', () => {
  it.each([
    ['USER', ['My Ideas', 'New Idea', 'Join requests']],
    ['ADMIN', ['All Ideas', 'Deadline', 'Statistics']],
  ] as const)('%s sees exactly navFor(role), in order', async (role, labels) => {
    await open('/ideas', role);
    const links = within(mainNav()).getAllByRole('link');
    expect(links.map((l) => l.textContent)).toEqual(labels);
    expect(links.map((l) => l.getAttribute('href'))).toEqual(navFor(role).map((r) => r.path));
  });

  it('marks only the current page with aria-current', async () => {
    await open('/join-requests');
    const links = within(mainNav()).getAllByRole('link');
    expect(links.filter((l) => l.getAttribute('aria-current') === 'page').map((l) => l.textContent)).toEqual(['Join requests']);
  });

  it('/ideas/new is the New Idea page, not the list', async () => {
    await open('/ideas/new');
    expect(within(mainNav()).getByRole('link', { name: 'New Idea' })).toHaveAttribute('aria-current', 'page');
    expect(within(mainNav()).getByRole('link', { name: 'My Ideas' })).not.toHaveAttribute('aria-current');
  });

  it('Admins get the Administrator badge; the email is never shown', async () => {
    await open('/ideas', 'ADMIN');
    expect(screen.getByText('Administrator')).toBeInTheDocument();
    expect(screen.getByText('陳主任')).toBeInTheDocument();
    expect(screen.getByText('000001')).toBeInTheDocument();
  });

  it('the skip link moves focus to main', async () => {
    const { user } = await open('/ideas');
    await user.click(screen.getByRole('link', { name: 'Skip to content' }));
    expect(screen.getByRole('main')).toHaveFocus();
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main');
  });
});

describe('sign out', () => {
  it('POSTs /auth/logout, clears cached queries and lands on /login without next', async () => {
    const posted = vi.fn();
    server.use(http.post(`${API}/auth/logout`, () => (posted(), new HttpResponse(null, { status: 204 }))));
    const { router, client, user } = await open('/ideas?page=2');
    await waitFor(() => expect(client.getQueryData(['deadline'])).toBeDefined());
    await user.click(screen.getByRole('button', { name: 'Sign out' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(router.state.location.search).toBe('');
    expect(posted).toHaveBeenCalledTimes(1);
    expect(client.getQueryData(['deadline'])).toBeUndefined();
    expect(client.getQueryData(['me'])).toBeNull();
    expect(await screen.findByRole('heading', { level: 1 })).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).not.toBeInTheDocument();
  });

  it('still lands on /login when the request fails, and sends it only once', async () => {
    const posted = vi.fn();
    server.use(
      http.post(`${API}/auth/logout`, async () => {
        posted();
        await delay(20);
        return HttpResponse.error();
      }),
    );
    const { router, user } = await open('/ideas');
    const button = screen.getByRole('button', { name: 'Sign out' });
    await user.click(button);
    expect(button).toBeDisabled();
    await user.click(button);
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(router.state.location.search).toBe('');
    expect(posted).toHaveBeenCalledTimes(1);
  });
});

describe('language switch', () => {
  afterEach(() => localStorage.clear());

  it('toggles <html lang> and the visible text', async () => {
    const { user } = await open('/ideas');
    expect(document.documentElement.lang).toBe('en');
    await user.click(screen.getByRole('button', { name: 'Switch language (EN / 中文)' }));
    await waitFor(() => expect(document.documentElement.lang).toBe('zh-TW'));
    expect(within(screen.getByRole('navigation', { name: '主要導覽' })).getByRole('link', { name: '我的提案' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '切換語言（EN / 中文）' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '切換語言（EN / 中文）' }));
    await waitFor(() => expect(document.documentElement.lang).toBe('en'));
    expect(within(mainNav()).getByRole('link', { name: 'My Ideas' })).toBeInTheDocument();
  });

  it('marks each part of the visible label with its language', async () => {
    await open('/ideas');
    const button = screen.getByRole('button', { name: 'Switch language (EN / 中文)' });
    expect(within(button).getByText('EN')).toHaveAttribute('lang', 'en');
    expect(within(button).getByText('中文')).toHaveAttribute('lang', 'zh-TW');
  });
});

describe('deadline pill', () => {
  it.each([
    ['NOT_OPEN', 'Submissions are not open yet.'],
    ['OPEN', 'Submissions are open.'],
    ['CLOSED', 'Submissions are closed.'],
    ['loading', 'Checking the submission status…'],
    ['error', 'The submission status could not be loaded.'],
  ] as const)('%s shows its state as text and passes axe', async (state, text) => {
    const { container } = await open('/ideas', 'USER', state);
    const pill = screen.getByRole('region', { name: 'Submission status' });
    await waitFor(() => expect(pill).toHaveAttribute('data-state', state));
    expect(within(pill).getByText(text)).toBeInTheDocument();
    expect(pill.querySelector('svg[aria-hidden="true"]')).not.toBeNull();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('shows the formatted deadline (no client-clock countdown)', async () => {
    await open('/ideas');
    const pill = screen.getByRole('region', { name: 'Submission status' });
    expect(await within(pill).findByText(/^Deadline: /)).toBeInTheDocument();
  });
});

describe('breadcrumb', () => {
  it.each([
    ['/ideas', 'USER', ['Contest', 'My Ideas'], []],
    ['/ideas', 'ADMIN', ['Administration', 'All Ideas'], []],
    ['/ideas/new', 'USER', ['Contest', 'My Ideas', 'Create an Idea'], ['My Ideas']],
    [`/ideas/${ID}`, 'USER', ['Contest', 'My Ideas', 'Idea'], ['My Ideas']],
    [`/ideas/${ID}`, 'ADMIN', ['Administration', 'All Ideas', 'Idea'], ['All Ideas']],
    [`/ideas/${ID}/edit`, 'USER', ['Contest', 'My Ideas', 'Edit Idea'], ['My Ideas']],
    ['/join-requests', 'USER', ['Contest', 'Join requests'], []],
    ['/admin/deadline', 'ADMIN', ['Administration', 'Deadline'], []],
    ['/admin/stats', 'ADMIN', ['Administration', 'Statistics'], []],
    ['/nope', 'USER', ['Contest', 'Page not found'], []],
    // a role mismatch is the same 404: the breadcrumb must not name the hidden page
    ['/admin/deadline', 'USER', ['Contest', 'Page not found'], []],
    ['/join-requests', 'ADMIN', ['Administration', 'Page not found'], []],
  ] as const)('%s as %s', async (path, role, crumbs, links) => {
    await open(path, role);
    const nav = await screen.findByRole('navigation', { name: 'Breadcrumb' });
    const items = within(nav).getAllByRole('listitem');
    expect(items.map((li) => li.textContent)).toEqual(crumbs);
    expect(within(nav).queryAllByRole('link').map((a) => a.textContent)).toEqual(links);
    const current = items.at(-1)!.querySelector('[aria-current="page"]');
    expect(current).toHaveTextContent(crumbs.at(-1)!);
  });
});

describe('drawer (below 1024 px)', () => {
  it('opens with the navigation, traps focus, closes on Esc and returns focus', async () => {
    const { user, container } = await open('/ideas');
    const trigger = screen.getByRole('button', { name: 'Open menu' });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    await user.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: 'Main navigation' });
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(trigger).toHaveAttribute('aria-controls', dialog.id);
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    expect(within(dialog).getAllByRole('link').map((l) => l.textContent)).toEqual(['My Ideas', 'New Idea', 'Join requests']);
    expect((await axe(container.ownerDocument.body)).violations).toEqual([]);
    for (let i = 0; i < 8; i++) {
      await user.tab();
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    }
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  it('closes when a link is followed', async () => {
    const { user, router } = await open('/ideas');
    await user.click(screen.getByRole('button', { name: 'Open menu' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('link', { name: 'Join requests' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/join-requests'));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
