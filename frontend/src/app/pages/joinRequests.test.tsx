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
const ID = '0193a1b2-0000-7000-8000-000000000001';

const jr = (over: Record<string, unknown> = {}) => ({
  ideaId: ID,
  ideaName: '成立供應商永續學院',
  category: 'COOL_IDEA',
  org: 'BSID',
  direction: 'TALENT_DEVELOPMENT',
  leader: { empNo: '089291', name: '李小華' },
  requestedAt: '2026-11-04T03:00:00.000Z',
  ...over,
});

async function open({ me = USER as { empNo: string; name: string; email: string | null; role: string }, items = [] as unknown[], state = 'OPEN', locale = 'en' } = {}) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state, serverTime: '2026-11-08T00:00:00.000Z' })),
    http.get(`${API}/join-requests`, () => HttpResponse.json({ items })),
  );
  await initI18n(locale as Locale);
  const router = createMemoryRouter(routes, { initialEntries: ['/join-requests'] });
  const view = render(<App router={router} client={new QueryClient()} />);
  return { user: userEvent.setup(), ...view };
}

describe('/join-requests', () => {
  it('lists my requests and has no axe violations', async () => {
    const { container } = await open({ items: [jr()] });
    const item = await within(await within(await screen.findByRole('main')).findByRole('region', { name: 'My pending requests' })).findByRole('listitem');
    expect(within(item).getByText('成立供應商永續學院')).toBeInTheDocument();
    expect(within(item).getByText(/Leader 李小華 · 089291/)).toBeInTheDocument();
    const time = item.querySelector('time');
    expect(time).toHaveAttribute('dateTime', '2026-11-04T03:00:00.000Z');
    expect(time).toHaveTextContent('Nov 4, 2026');
    expect((await axe(container)).violations).toEqual([]);
  });

  it('shows the empty state', async () => {
    await open();
    expect(await screen.findByText('You have no pending requests.')).toBeInTheDocument();
  });

  it('shows the 404 page to an Admin', async () => {
    await open({ me: ADMIN });
    expect(await screen.findByRole('heading', { level: 1, name: /not found/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Send request' })).not.toBeInTheDocument();
  });

  it('pre-checks the form without calling the server', async () => {
    const posted = vi.fn();
    server.use(http.post(`${API}/join-requests`, () => (posted(), HttpResponse.json(jr(), { status: 201 }))));
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Send request' }));
    expect(await screen.findAllByText('This field is required.')).toHaveLength(2);
    await user.type(screen.getByLabelText('Leader employee ID'), '12ab');
    await user.type(screen.getByLabelText('Idea name'), 'x');
    await user.click(screen.getByRole('button', { name: 'Send request' }));
    expect(await screen.findByText('The format is not valid.')).toBeInTheDocument();
    expect(posted).not.toHaveBeenCalled();
  });

  it('sends the request and lists it', async () => {
    let body: unknown;
    let csrf: string | null = null;
    server.use(
      http.post(`${API}/join-requests`, async ({ request }) => {
        body = await request.json();
        csrf = request.headers.get('X-CSRF-Token');
        return HttpResponse.json(jr(), { status: 201 });
      }),
    );
    const { user } = await open();
    await screen.findByText('You have no pending requests.');
    await user.type(screen.getByLabelText('Leader employee ID'), ' 089291 ');
    await user.type(screen.getByLabelText('Idea name'), '  成立供應商永續學院 ');
    await user.click(screen.getByRole('button', { name: 'Send request' }));
    expect(await screen.findByText(/Your request was sent/)).toBeInTheDocument();
    expect(body).toEqual({ leaderEmpNo: '089291', ideaName: '成立供應商永續學院' });
    expect(csrf).toBe('tok');
    expect(await within(within(screen.getByRole('main')).getByRole('region', { name: 'My pending requests' })).findByRole('listitem')).toHaveTextContent('成立供應商永續學院');
    expect(screen.getByLabelText('Idea name')).toHaveValue('');
  });

  it.each([
    [409, 'JOIN_REQUEST_EXISTS', 'You already have a join request for this Idea.'],
    [409, 'ALREADY_MEMBER', 'You are already a member of this Idea.'],
    [422, 'CANNOT_JOIN_OWN_IDEA', 'You cannot request to join your own Idea.'],
    [403, 'SUBMISSION_CLOSED', 'Submissions are closed.'],
  ])('shows %i %s', async (status, code, text) => {
    server.use(http.post(`${API}/join-requests`, () => problem(status, code)));
    const { user } = await open();
    await screen.findByText('You have no pending requests.');
    await user.type(screen.getByLabelText('Leader employee ID'), '089291');
    await user.type(screen.getByLabelText('Idea name'), 'abc');
    await user.click(screen.getByRole('button', { name: 'Send request' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(text);
  });

  it('maps a server field error to the field', async () => {
    server.use(http.post(`${API}/join-requests`, () => problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'ideaName', code: 'OUT_OF_RANGE' }] })));
    const { user } = await open();
    await screen.findByText('You have no pending requests.');
    await user.type(screen.getByLabelText('Leader employee ID'), '089291');
    await user.type(screen.getByLabelText('Idea name'), 'abc');
    await user.click(screen.getByRole('button', { name: 'Send request' }));
    expect(await screen.findByText('The value is out of range.')).toBeInTheDocument();
    expect(screen.getByLabelText('Idea name')).toHaveAttribute('aria-invalid', 'true');
  });

  it('withdraws by Idea ID', async () => {
    let path = '';
    let csrf: string | null = null;
    server.use(
      http.delete(`${API}/ideas/:id/members/me`, ({ request }) => {
        path = new URL(request.url).pathname;
        csrf = request.headers.get('X-CSRF-Token');
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = await open({ items: [jr()] });
    await user.click(await screen.findByRole('button', { name: 'Withdraw request for 成立供應商永續學院' }));
    await waitFor(() => expect(screen.getByText('You have no pending requests.')).toBeInTheDocument());
    expect(path).toBe(`/api/v1/ideas/${ID}/members/me`);
    expect(csrf).toBe('tok');
  });

  it('refreshes the list when the withdraw answers 404', async () => {
    server.use(http.delete(`${API}/ideas/:id/members/me`, () => problem(404, 'NOT_FOUND')));
    let calls = 0;
    server.use(http.get(`${API}/join-requests`, () => HttpResponse.json({ items: calls++ === 0 ? [jr()] : [] })));
    const { user } = await open({ items: [jr()] });
    server.use(http.get(`${API}/join-requests`, () => HttpResponse.json({ items: calls++ === 0 ? [jr()] : [] })));
    await user.click(await screen.findByRole('button', { name: /Withdraw request/ }));
    await waitFor(() => expect(screen.getByText('You have no pending requests.')).toBeInTheDocument());
  });

  it('disables the controls outside the submission window', async () => {
    await open({ items: [jr()], state: 'CLOSED' });
    await screen.findByText(/Submissions are not open/);
    expect(screen.getByRole('button', { name: 'Send request' })).toBeDisabled();
    expect(await screen.findByRole('button', { name: /Withdraw request/ })).toBeDisabled();
  });

  it('renders in zh-TW', async () => {
    await open({ items: [jr()], locale: 'zh-TW' });
    expect(await screen.findByRole('button', { name: '送出申請' })).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: '撤回「成立供應商永續學院」的申請' })).toBeInTheDocument();
  });

  it('shows the uniform 404 under the Idea name field without blaming either input', async () => {
    server.use(http.post(`${API}/join-requests`, () => problem(404, 'NOT_FOUND')));
    const { user, container } = await open();
    await screen.findByText('You have no pending requests.');
    await user.type(screen.getByLabelText('Leader employee ID'), '089291');
    await user.type(screen.getByLabelText('Idea name'), 'abc');
    await user.click(screen.getByRole('button', { name: 'Send request' }));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('No Idea matches this Leader ID and name. Check the exact spelling.');
    const name = screen.getByLabelText('Idea name');
    expect(name).toHaveAttribute('aria-invalid', 'true');
    expect(name.getAttribute('aria-describedby')?.split(' ')).toContain(alert.id);
    expect(screen.getByLabelText('Leader employee ID')).not.toHaveAttribute('aria-invalid');
    expect((await axe(container)).violations).toEqual([]);
  });

  it('shows a count badge that matches the list length', async () => {
    await open({ items: [jr(), jr({ ideaId: '0193a1b2-0000-7000-8000-000000000002', ideaName: 'Green commute' }), jr({ ideaId: '0193a1b2-0000-7000-8000-000000000003', ideaName: 'Audit' })] });
    const region = await screen.findByRole('region', { name: 'My pending requests' });
    expect(await within(region).findAllByRole('listitem')).toHaveLength(3);
    expect(within(region).getByText('3 pending')).toBeInTheDocument();
  });

  it('hides the count badge when the list is empty and passes axe', async () => {
    const { container } = await open();
    await screen.findByText('You have no pending requests.');
    expect(screen.queryByText(/\d+ pending/)).not.toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('renders the explainer as an ordered list', async () => {
    await open();
    const region = await screen.findByRole('region', { name: 'How it works' });
    const list = within(region).getByRole('list');
    expect(list.tagName).toBe('OL');
    const steps = within(list).getAllByRole('listitem');
    expect(steps).toHaveLength(3);
    expect(steps[0]).toHaveTextContent('Apply');
    expect(steps[1]).toHaveTextContent('Leader reviews');
    expect(steps[2]).toHaveTextContent("You're in");
  });

  it('disables every apply and withdraw control when CLOSED', async () => {
    await open({ items: [jr(), jr({ ideaId: '0193a1b2-0000-7000-8000-000000000002', ideaName: 'Green commute' })], state: 'CLOSED' });
    await screen.findByText(/Submissions are not open/);
    expect(screen.getByRole('button', { name: 'Send request' })).toBeDisabled();
    const withdraws = await screen.findAllByRole('button', { name: /Withdraw request/ });
    expect(withdraws).toHaveLength(2);
    for (const b of withdraws) expect(b).toBeDisabled();
  });
});
