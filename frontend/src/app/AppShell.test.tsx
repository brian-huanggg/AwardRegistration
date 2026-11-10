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
  );
  await initI18n('en');
  const client = new QueryClient();
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const view = render(<App router={router} client={client} />);
  await screen.findByRole('heading', { level: 1 });
  return { router, client, user: userEvent.setup(), ...view };
}

const mainNav = () => screen.getByRole('navigation', { name: 'Main navigation' });

describe('navbar navigation (Frontend.md §6.2, §6.5; Eco Navbar)', () => {
  it.each([
    ['USER', ['My Ideas', 'New Idea']],
    ['ADMIN', ['Registrations', 'Deadline', 'Statistics']],
  ] as const)('%s sees exactly navFor(role), in order', async (role, labels) => {
    await open('/ideas', role);
    const links = within(mainNav()).getAllByRole('link');
    expect(links.map((l) => l.textContent)).toEqual(labels);
    expect(links.map((l) => l.getAttribute('href'))).toEqual(navFor(role).map((r) => r.path));
  });

  it('marks only the current page with aria-current', async () => {
    await open('/admin/deadline', 'ADMIN');
    const links = within(mainNav()).getAllByRole('link');
    expect(links.filter((l) => l.getAttribute('aria-current') === 'page').map((l) => l.textContent)).toEqual(['Deadline']);
  });

  it('/join-requests no longer exists: a User gets the 404 page', async () => {
    await open('/join-requests');
    expect(screen.getByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
  });

  describe('Guide link (VITE_GUIDE_URL)', () => {
    afterEach(() => vi.unstubAllEnvs());

    it('is hidden when the env var is unset', async () => {
      vi.stubEnv('VITE_GUIDE_URL', '');
      await open('/ideas');
      expect(within(mainNav()).queryByRole('link', { name: /Guide/ })).not.toBeInTheDocument();
    });

    it('is an external link for Users, opening in a new tab', async () => {
      vi.stubEnv('VITE_GUIDE_URL', 'https://example.com/guide');
      await open('/ideas');
      const guide = within(mainNav()).getByRole('link', { name: /^Guide/ });
      expect(guide).toHaveTextContent('(opens in a new tab)');
      expect(guide).toHaveAttribute('href', 'https://example.com/guide');
      expect(guide).toHaveAttribute('target', '_blank');
      expect(guide).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('is not shown to Admins', async () => {
      vi.stubEnv('VITE_GUIDE_URL', 'https://example.com/guide');
      await open('/ideas', 'ADMIN');
      expect(within(mainNav()).queryByRole('link', { name: /Guide/ })).not.toBeInTheDocument();
    });
  });

  it('/ideas/new is the New Idea page, not the list', async () => {
    await open('/ideas/new');
    expect(within(mainNav()).getByRole('link', { name: 'New Idea' })).toHaveAttribute('aria-current', 'page');
    expect(within(mainNav()).getByRole('link', { name: 'My Ideas' })).not.toHaveAttribute('aria-current');
  });

  it('the user menu shows name, employee ID and role; the email is never shown', async () => {
    await open('/ideas', 'ADMIN');
    const menu = screen.getByRole('button', { name: /陳主任/ });
    expect(within(menu).getByText('Administrator')).toBeInTheDocument();
    expect(within(menu).getByText('000001')).toBeInTheDocument();
    expect(screen.queryByText(/@/)).not.toBeInTheDocument();
  });

  it('a User is labelled User', async () => {
    await open('/ideas');
    expect(within(screen.getByRole('button', { name: /王小明/ })).getByText('User')).toBeInTheDocument();
  });

  it('the user menu opens a panel with Sign out, closes on Esc and returns focus', async () => {
    const { user, container } = await open('/ideas');
    const trigger = screen.getByRole('button', { name: /王小明/ });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument();
    await user.click(trigger);
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeVisible();
    expect((await axe(container)).violations).toEqual([]);
    await user.tab();
    await user.keyboard('{Escape}');
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(trigger).toHaveFocus();
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
    await user.click(screen.getByRole('button', { name: /王小明/ }));
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
    await user.click(screen.getByRole('button', { name: /王小明/ }));
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

describe('active navbar item (replaces the breadcrumb)', () => {
  it.each([
    ['/ideas', 'USER', 'My Ideas', 'My Ideas'],
    ['/ideas', 'ADMIN', 'Registrations', 'Registrations'],
    ['/ideas/new', 'USER', 'New Idea', 'New Idea'],
    // detail and edit belong to the list: underlined, but not the current page
    [`/ideas/${ID}`, 'USER', 'My Ideas', null],
    [`/ideas/${ID}`, 'ADMIN', 'Registrations', null],
    [`/ideas/${ID}/edit`, 'USER', 'My Ideas', null],
    ['/admin/deadline', 'ADMIN', 'Deadline', 'Deadline'],
    ['/admin/stats', 'ADMIN', 'Statistics', 'Statistics'],
    ['/nope', 'USER', null, null],
    // a role mismatch is the same 404: no item reveals the hidden page
    ['/admin/deadline', 'USER', null, null],
  ] as const)('%s as %s', async (path, role, underlined, current) => {
    await open(path, role);
    const links = within(mainNav()).getAllByRole('link');
    expect(links.filter((l) => l.className.includes('border-brand-ink')).map((l) => l.textContent)).toEqual(underlined ? [underlined] : []);
    expect(links.filter((l) => l.getAttribute('aria-current') === 'page').map((l) => l.textContent)).toEqual(current ? [current] : []);
  });
});

describe('mobile menu (below 1024 px)', () => {
  it('opens with the navigation, traps focus, closes on Esc and returns focus', async () => {
    const { user, container } = await open('/ideas');
    const trigger = screen.getByRole('button', { name: 'Open menu' });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    await user.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: 'Main navigation' });
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(trigger).toHaveAttribute('aria-controls', dialog.id);
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    expect(within(dialog).getAllByRole('link').map((l) => l.textContent)).toEqual(['My Ideas', 'New Idea']);
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
    await user.click(within(dialog).getByRole('link', { name: 'New Idea' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/ideas/new'));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });
});
