import { QueryClient } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter } from 'react-router';
import { axe } from 'vitest-axe';
import { clearCsrfToken } from '@/shared/api';
import { problem } from '@/shared/api/mocks/problem';
import { server } from '@/shared/api/mocks/server';
import { initI18n } from '@/shared/i18n';
import { App } from '../App';
import { routes } from '../router';

const API = 'http://localhost:3000/api/v1';
const ID = '0193a1b2-0000-7000-8000-000000000001';
const LEADER = { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' };
const MEMBER = { empNo: '089291', name: '李小華', email: 'lee@example.com', role: 'USER' };

const detail = (over: Record<string, unknown> = {}) => ({
  id: ID,
  name: '成立供應商永續學院',
  category: 'COOL_IDEA',
  org: 'BSID',
  direction: 'TALENT_DEVELOPMENT',
  summary: 'line one',
  leader: { empNo: '089290', name: '王小明' },
  members: [{ empNo: '089291', name: '李小華', joinedAt: '2026-11-03T03:00:00.000Z' }],
  pendingRequests: [],
  slides: { ext: 'pdf', size: 2048, sha256: 'x', updatedAt: '2026-11-02T03:00:00.000Z', downloadFilename: 'a.pdf' },
  expectedFilenameBase: 'I_BSID_王小明_089290_成立供應商永續學院',
  createdAt: '2026-11-02T03:00:00.000Z',
  updatedAt: '2026-11-02T03:00:00.000Z',
  version: 1,
  ...over,
});

async function open({ me = LEADER, state = 'OPEN', path = `/ideas/${ID}/edit` } = {}) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state, serverTime: '2026-11-08T00:00:00.000Z' })),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
  );
  await initI18n('en');
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const view = render(<App router={router} client={new QueryClient()} />);
  return { router, user: userEvent.setup(), ...view };
}

const idea = (over: Record<string, unknown> = {}) => server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail(over))));

describe('Edit Idea page', () => {
  it('starts from the current values, has no slide file, and passes axe', async () => {
    idea();
    const { container } = await open();
    expect(await screen.findByLabelText('Idea name')).toHaveValue('成立供應商永續學院');
    expect(screen.getByRole('heading', { level: 1, name: 'Edit Idea' })).toBeInTheDocument();
    expect(screen.getByLabelText('Category')).toHaveValue('COOL_IDEA');
    expect(screen.getByLabelText('Summary')).toHaveValue('line one');
    expect(screen.queryByLabelText('Slide file')).not.toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('sends the five text fields with PUT and returns to the detail without another GET', async () => {
    let gets = 0;
    let sent: { body: unknown; csrf: string | null } | null = null;
    server.use(
      http.get(`${API}/ideas/${ID}`, () => (gets++, HttpResponse.json(detail()))),
      http.put(`${API}/ideas/${ID}`, async ({ request }) => {
        sent = { body: await request.json(), csrf: request.headers.get('X-CSRF-Token') };
        return HttpResponse.json(detail({ name: 'New name', version: 2 }));
      }),
    );
    const { router, user } = await open();
    const name = await screen.findByLabelText('Idea name');
    await user.clear(name);
    await user.type(name, '  New name ');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(router.state.location.pathname).toBe(`/ideas/${ID}`));
    expect(sent).toEqual({
      csrf: 'tok',
      body: { category: 'COOL_IDEA', org: 'BSID', direction: 'TALENT_DEVELOPMENT', name: 'New name', summary: 'line one' },
    });
    expect(await screen.findByRole('heading', { level: 1, name: 'New name' })).toBeInTheDocument();
    expect(gets).toBe(1);
  });

  it('pre-checks locally: an empty name is not sent', async () => {
    idea();
    let puts = 0;
    server.use(http.put(`${API}/ideas/${ID}`, () => (puts++, HttpResponse.json(detail()))));
    const { user } = await open();
    await user.clear(await screen.findByLabelText('Idea name'));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Idea name');
    expect(screen.getByLabelText('Idea name')).toHaveAttribute('aria-invalid', 'true');
    expect(puts).toBe(0);
  });

  it('maps 409 IDEA_DUPLICATE to the name field', async () => {
    idea();
    server.use(http.put(`${API}/ideas/${ID}`, () => problem(409, 'IDEA_DUPLICATE')));
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Save' }));
    await waitFor(() => expect(screen.getByLabelText('Idea name')).toHaveAttribute('aria-invalid', 'true'));
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('shows a request-level error (503) with the form kept for another try', async () => {
    idea();
    server.use(http.put(`${API}/ideas/${ID}`, () => problem(503, 'SERVICE_UNAVAILABLE')));
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled();
  });

  it('disables Save outside the submission window', async () => {
    idea();
    await open({ state: 'CLOSED' });
    await screen.findByLabelText('Idea name');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled());
    expect(screen.getByText(/cannot be changed now/)).toBeInTheDocument();
  });

  it('sends a Member (not the Leader) to the detail page', async () => {
    idea();
    const { router } = await open({ me: MEMBER });
    await waitFor(() => expect(router.state.location.pathname).toBe(`/ideas/${ID}`));
  });

  it('shows the one 404 page when the Idea is hidden or missing', async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => problem(404, 'NOT_FOUND')));
    await open();
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
  });

  it('Cancel goes back to the detail page', async () => {
    idea();
    const { router, user } = await open();
    await user.click(await screen.findByRole('link', { name: 'Cancel' }));
    expect(router.state.location.pathname).toBe(`/ideas/${ID}`);
  });
});

describe('Edit link on the detail page', () => {
  it('is offered to the Leader only', async () => {
    idea();
    const a = await open({ path: `/ideas/${ID}` });
    expect(await screen.findByRole('link', { name: 'Edit details' })).toHaveAttribute('href', `/ideas/${ID}/edit`);
    a.unmount();
    idea();
    await open({ me: MEMBER, path: `/ideas/${ID}` });
    await screen.findByRole('heading', { level: 1, name: '成立供應商永續學院' });
    expect(screen.queryByRole('link', { name: 'Edit details' })).not.toBeInTheDocument();
  });
});
