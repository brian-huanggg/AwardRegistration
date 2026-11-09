import { QueryClient } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
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
const ADMIN = { empNo: '000001', name: 'Admin', email: null, role: 'ADMIN' };

const detail = (over: Record<string, unknown> = {}) => ({
  id: ID,
  name: '成立供應商永續學院',
  category: 'COOL_IDEA',
  org: 'BSID',
  direction: 'TALENT_DEVELOPMENT',
  summary: 's',
  leader: { empNo: '089290', name: '王小明' },
  members: [{ empNo: '089291', name: '李小華', joinedAt: '2026-11-03T03:00:00.000Z' }],
  pendingRequests: [{ empNo: '089292', name: '陳大文', requestedAt: '2026-11-04T03:00:00.000Z' }],
  slides: { ext: 'pdf', size: 2048, sha256: 'x', updatedAt: '2026-11-02T03:00:00.000Z', downloadFilename: 'a.pdf' },
  expectedFilenameBase: 'a',
  createdAt: '2026-11-02T03:00:00.000Z',
  updatedAt: '2026-11-02T03:00:00.000Z',
  version: 1,
  ...over,
});

async function open(me: typeof LEADER | typeof ADMIN, state = 'OPEN') {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state, serverTime: '2026-11-08T00:00:00.000Z' })),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 't' })),
  );
  await initI18n('en');
  const router = createMemoryRouter(routes, { initialEntries: [`/ideas/${ID}`] });
  const view = render(<App router={router} client={new QueryClient()} />);
  return { user: userEvent.setup(), router, ...view };
}

describe('Member management', () => {
  it('Leader approves a request: the response replaces the page data', async () => {
    let calls = 0;
    server.use(
      http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())),
      http.post(`${API}/ideas/${ID}/members/089292/approve`, () => {
        calls++;
        return HttpResponse.json(
          detail({
            members: [
              { empNo: '089291', name: '李小華', joinedAt: '2026-11-03T03:00:00.000Z' },
              { empNo: '089292', name: '陳大文', joinedAt: '2026-11-05T03:00:00.000Z' },
            ],
            pendingRequests: [],
          }),
        );
      }),
    );
    const { user, container } = await open(LEADER);
    expect((await axe(await screen.findByText('李小華 (089291)').then(() => container))).violations).toEqual([]);
    await user.click(await screen.findByRole('button', { name: 'Approve 陳大文 (089292)' }));
    expect(await screen.findByText('陳大文 (089292)')).toBeInTheDocument();
    expect(screen.getByText('No pending requests.')).toBeInTheDocument();
    expect(calls).toBe(1);
  });

  it('approve answering TEAM_FULL shows the message and keeps the page usable', async () => {
    server.use(
      http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())),
      http.post(`${API}/ideas/${ID}/members/089292/approve`, () => problem(409, 'TEAM_FULL')),
    );
    const { user } = await open(LEADER);
    await user.click(await screen.findByRole('button', { name: /Approve 陳大文/ }));
    expect(await screen.findByRole('alert')).toHaveTextContent('This team is full.');
  });

  it('rejecting asks for confirmation; Cancel sends nothing', async () => {
    let calls = 0;
    server.use(
      http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())),
      http.delete(`${API}/ideas/${ID}/members/089292`, () => (calls++, new HttpResponse(null, { status: 204 }))),
    );
    const { user } = await open(LEADER);
    await user.click(await screen.findByRole('button', { name: /Reject 陳大文/ }));
    const dialog = await screen.findByRole('alertdialog');
    expect((await axe(dialog)).violations).toEqual([]);
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(calls).toBe(0);
  });

  it('confirmed reject refetches the Idea', async () => {
    let pending = true;
    server.use(
      http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail(pending ? {} : { pendingRequests: [] }))),
      http.delete(`${API}/ideas/${ID}/members/089292`, () => ((pending = false), new HttpResponse(null, { status: 204 }))),
    );
    const { user } = await open(LEADER);
    await user.click(await screen.findByRole('button', { name: /Reject 陳大文/ }));
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Reject request' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(await screen.findByText('No pending requests.')).toBeInTheDocument();
  });

  it('removing a Member needs confirmation and an error stays in the dialog', async () => {
    server.use(
      http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())),
      http.delete(`${API}/ideas/${ID}/members/089291`, () => problem(503, 'SERVICE_UNAVAILABLE')),
    );
    const { user } = await open(LEADER);
    await user.click(await screen.findByRole('button', { name: /Remove 李小華/ }));
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Remove member' }));
    expect(await within(dialog).findByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('alertdialog')).toBeInTheDocument();
  });

  it('a Member can leave and lands on the list', async () => {
    let left = false;
    server.use(
      http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail({ pendingRequests: null }))),
      http.get(`${API}/ideas`, () => HttpResponse.json({ items: [], page: 1, size: 20, total: 0 })),
      http.delete(`${API}/ideas/${ID}/members/me`, () => ((left = true), new HttpResponse(null, { status: 204 }))),
    );
    const { user, router } = await open(MEMBER);
    expect(screen.queryByRole('button', { name: /Remove|Approve|Reject/ })).not.toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: 'Leave team' }));
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Leave team' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/ideas'));
    expect(left).toBe(true);
  });

  it('a Leader sees no Leave button; an Admin sees no member controls', async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())));
    const a = await open(LEADER);
    await screen.findByText('李小華 (089291)');
    expect(screen.queryByRole('button', { name: 'Leave team' })).not.toBeInTheDocument();
    a.unmount();
    await open(ADMIN);
    await screen.findByText('李小華 (089291)');
    expect(screen.queryByRole('button', { name: /Approve|Reject|Remove|Leave/ })).not.toBeInTheDocument();
  });

  it('controls are disabled outside the submission window', async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())));
    await open(LEADER, 'CLOSED');
    await screen.findByText('李小華 (089291)');
    for (const n of [/Approve/, /Reject/, /Remove/]) expect(await screen.findByRole('button', { name: n })).toBeDisabled();
  });
});
