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

/** Opens My Ideas, then the "Join a team" dialog (owner decision 2026-11-11: replaces /join-requests). */
async function openDialog({ items = [] as unknown[], state = 'OPEN', locale = 'en' as Locale } = {}) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(USER)),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state, serverTime: '2026-11-08T00:00:00.000Z' })),
    http.get(`${API}/ideas`, () => HttpResponse.json({ items: [], page: 1, size: 20, total: 0 })),
    http.get(`${API}/join-requests`, () => HttpResponse.json({ items })),
  );
  await initI18n(locale);
  const router = createMemoryRouter(routes, { initialEntries: ['/ideas'] });
  const view = render(<App router={router} client={new QueryClient()} />);
  const user = userEvent.setup();
  await screen.findByRole('heading', { level: 1 });
  const trigger = within(screen.getByRole('main')).getByRole('button', { name: locale === 'en' ? 'Join a team' : /加入/ });
  await user.click(trigger);
  const dialog = await screen.findByRole('dialog');
  return { user, trigger, dialog, ...view };
}

const pending = (dialog: HTMLElement) => within(dialog).getByRole('region', { name: 'My pending requests' });

describe('Join-a-team dialog on My Ideas', () => {
  it('opens a titled modal with focus inside, lists my requests, passes axe, and Esc returns focus', async () => {
    const { user, trigger, dialog } = await openDialog({ items: [jr()] });
    expect(dialog).toHaveAccessibleName('Join a team');
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    const item = await within(pending(dialog)).findByRole('listitem');
    expect(within(item).getByText('成立供應商永續學院')).toBeInTheDocument();
    expect(within(item).getByText(/Leader 李小華 · 089291/)).toBeInTheDocument();
    const time = item.querySelector('time');
    expect(time).toHaveAttribute('dateTime', '2026-11-04T03:00:00.000Z');
    expect(time).toHaveTextContent('Nov 4, 2026');
    expect(within(pending(dialog)).getByText('1 pending')).toBeInTheDocument();
    expect((await axe(document.body)).violations).toEqual([]);
    for (let i = 0; i < 8; i++) {
      await user.tab();
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    }
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  it('shows the empty state and no count badge', async () => {
    const { dialog } = await openDialog();
    expect(await within(dialog).findByText('You have no pending requests.')).toBeInTheDocument();
    expect(within(dialog).queryByText(/\d+ pending/)).not.toBeInTheDocument();
  });

  it('pre-checks the form without calling the server', async () => {
    const posted = vi.fn();
    server.use(http.post(`${API}/join-requests`, () => (posted(), HttpResponse.json(jr(), { status: 201 }))));
    const { user, dialog } = await openDialog();
    await user.click(within(dialog).getByRole('button', { name: 'Send request' }));
    expect(await within(dialog).findAllByText('This field is required.')).toHaveLength(2);
    await user.type(within(dialog).getByLabelText('Leader employee ID'), '12ab');
    await user.type(within(dialog).getByLabelText('Idea name'), 'x');
    await user.click(within(dialog).getByRole('button', { name: 'Send request' }));
    expect(await within(dialog).findByText('The format is not valid.')).toBeInTheDocument();
    expect(posted).not.toHaveBeenCalled();
  });

  it('submits the request and lists it', async () => {
    let body: unknown;
    let csrf: string | null = null;
    server.use(
      http.post(`${API}/join-requests`, async ({ request }) => {
        body = await request.json();
        csrf = request.headers.get('X-CSRF-Token');
        return HttpResponse.json(jr(), { status: 201 });
      }),
    );
    const { user, dialog } = await openDialog();
    await within(dialog).findByText('You have no pending requests.');
    await user.type(within(dialog).getByLabelText('Leader employee ID'), ' 089291 ');
    await user.type(within(dialog).getByLabelText('Idea name'), '  成立供應商永續學院 ');
    await user.click(within(dialog).getByRole('button', { name: 'Send request' }));
    expect(await within(dialog).findByText(/Your request was sent/)).toBeInTheDocument();
    expect(body).toEqual({ leaderEmpNo: '089291', ideaName: '成立供應商永續學院' });
    expect(csrf).toBe('tok');
    expect(await within(pending(dialog)).findByRole('listitem')).toHaveTextContent('成立供應商永續學院');
    expect(within(dialog).getByLabelText('Idea name')).toHaveValue('');
  });

  it.each([
    [409, 'JOIN_REQUEST_EXISTS', 'You already have a join request for this Idea.'],
    [409, 'ALREADY_MEMBER', 'You are already a member of this Idea.'],
    [422, 'CANNOT_JOIN_OWN_IDEA', 'You cannot request to join your own Idea.'],
    [403, 'SUBMISSION_CLOSED', 'Submissions are closed.'],
  ])('shows %i %s', async (status, code, text) => {
    server.use(http.post(`${API}/join-requests`, () => problem(status, code)));
    const { user, dialog } = await openDialog();
    await within(dialog).findByText('You have no pending requests.');
    await user.type(within(dialog).getByLabelText('Leader employee ID'), '089291');
    await user.type(within(dialog).getByLabelText('Idea name'), 'abc');
    await user.click(within(dialog).getByRole('button', { name: 'Send request' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(text);
  });

  it('maps a server field error to the field', async () => {
    server.use(http.post(`${API}/join-requests`, () => problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'ideaName', code: 'OUT_OF_RANGE' }] })));
    const { user, dialog } = await openDialog();
    await within(dialog).findByText('You have no pending requests.');
    await user.type(within(dialog).getByLabelText('Leader employee ID'), '089291');
    await user.type(within(dialog).getByLabelText('Idea name'), 'abc');
    await user.click(within(dialog).getByRole('button', { name: 'Send request' }));
    expect(await within(dialog).findByText('The value is out of range.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('Idea name')).toHaveAttribute('aria-invalid', 'true');
  });

  it('shows the uniform 404 under the Idea name field without blaming either input, and passes axe', async () => {
    server.use(http.post(`${API}/join-requests`, () => problem(404, 'NOT_FOUND')));
    const { user, dialog } = await openDialog();
    await within(dialog).findByText('You have no pending requests.');
    await user.type(within(dialog).getByLabelText('Leader employee ID'), '089291');
    await user.type(within(dialog).getByLabelText('Idea name'), 'abc');
    await user.click(within(dialog).getByRole('button', { name: 'Send request' }));
    const alert = await within(dialog).findByRole('alert');
    expect(alert).toHaveTextContent('No Idea matches this Leader ID and name. Check the exact spelling.');
    const name = within(dialog).getByLabelText('Idea name');
    expect(name).toHaveAttribute('aria-invalid', 'true');
    expect(name.getAttribute('aria-describedby')?.split(' ')).toContain(alert.id);
    expect(within(dialog).getByLabelText('Leader employee ID')).not.toHaveAttribute('aria-invalid');
    expect((await axe(document.body)).violations).toEqual([]);
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
    const { user, dialog } = await openDialog({ items: [jr()] });
    await user.click(await within(dialog).findByRole('button', { name: 'Withdraw request for 成立供應商永續學院' }));
    await waitFor(() => expect(within(dialog).getByText('You have no pending requests.')).toBeInTheDocument());
    expect(path).toBe(`/api/v1/ideas/${ID}/members/me`);
    expect(csrf).toBe('tok');
  });

  it('refreshes the list when the withdraw answers 404', async () => {
    server.use(http.delete(`${API}/ideas/:id/members/me`, () => problem(404, 'NOT_FOUND')));
    const { user, dialog } = await openDialog({ items: [jr()] });
    let calls = 0;
    server.use(http.get(`${API}/join-requests`, () => (calls++, HttpResponse.json({ items: [] }))));
    await user.click(await within(dialog).findByRole('button', { name: /Withdraw request/ }));
    await waitFor(() => expect(within(dialog).getByText('You have no pending requests.')).toBeInTheDocument());
    expect(calls).toBeGreaterThan(0);
  });

  it('disables every apply and withdraw control outside the submission window, and passes axe', async () => {
    const { dialog } = await openDialog({ items: [jr(), jr({ ideaId: '0193a1b2-0000-7000-8000-000000000002', ideaName: 'Green commute' })], state: 'CLOSED' });
    expect(await within(dialog).findByText(/Submissions are not open/)).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Send request' })).toBeDisabled();
    const withdraws = await within(dialog).findAllByRole('button', { name: /Withdraw request/ });
    expect(withdraws).toHaveLength(2);
    for (const b of withdraws) expect(b).toBeDisabled();
    expect((await axe(document.body)).violations).toEqual([]);
  });

  it('renders in zh-TW', async () => {
    const { dialog } = await openDialog({ items: [jr()], locale: 'zh-TW' });
    expect(await within(dialog).findByRole('button', { name: '送出申請' })).toBeInTheDocument();
    expect(await within(dialog).findByRole('button', { name: '撤回「成立供應商永續學院」的申請' })).toBeInTheDocument();
  });
});
