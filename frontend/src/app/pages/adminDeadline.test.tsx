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
const ADMIN = { empNo: 'A0001', name: 'Admin', email: null, role: 'ADMIN' };
const USER = { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' };
const body = (state = 'OPEN', deadline: string | null = '2026-12-01T02:30:00.000Z') => ({ deadline, state, serverTime: '2026-11-08T00:00:00.000Z' });

let gets = 0;

async function open(me: Record<string, unknown> = ADMIN, current = body()) {
  gets = 0;
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => (gets++, HttpResponse.json(current))),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
  );
  await initI18n('en');
  const router = createMemoryRouter(routes, { initialEntries: ['/admin/deadline'] });
  const view = render(<App router={router} client={new QueryClient()} />);
  return { router, user: userEvent.setup(), ...view };
}

describe('Admin deadline page', () => {
  it('shows the current state, prefills the form and passes axe', async () => {
    const { container } = await open();
    const input = await screen.findByLabelText('Submission deadline');
    expect(screen.getByRole('heading', { level: 1, name: 'Deadline' })).toBeInTheDocument();
    await waitFor(() => expect((input as HTMLInputElement).value).toMatch(/^2026-12-\d{2}T\d{2}:\d{2}(:\d{2})?$/));
    expect(await screen.findByText(/Submissions are open/)).toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('a User sees the 404 page', async () => {
    await open(USER);
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
    expect(screen.queryByLabelText('Submission deadline')).not.toBeInTheDocument();
  });

  it('PUTs ISO 8601 with offset and CSRF, then shows the new state without another GET', async () => {
    let sent: { body: unknown; csrf: string | null } | null = null;
    server.use(
      http.put(`${API}/admin/deadline`, async ({ request }) => {
        sent = { body: await request.json(), csrf: request.headers.get('X-CSRF-Token') };
        return HttpResponse.json(body('OPEN', '2026-12-31T15:00:00.000Z'));
      }),
    );
    const { user } = await open(ADMIN, body('NOT_OPEN', null));
    const input = await screen.findByLabelText('Submission deadline');
    await user.type(input, '2026-12-31T23:00:00');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByText('The deadline was saved.')).toBeInTheDocument();
    expect(sent!.csrf).toBe('tok');
    expect((sent!.body as { deadline: string }).deadline).toMatch(/^2026-12-31T23:00:00[+-]\d{2}:\d{2}$/);
    expect(await screen.findByText(/Submissions are open/)).toBeInTheDocument();
    expect(gets).toBe(1);
  });

  it('maps a 422 OUT_OF_RANGE to the field and focuses the summary', async () => {
    server.use(
      http.put(`${API}/admin/deadline`, () =>
        problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'deadline', code: 'OUT_OF_RANGE' }] }),
      ),
    );
    const { user } = await open();
    const input = await screen.findByLabelText('Submission deadline');
    await waitFor(() => expect(input).not.toHaveValue(''));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    const alerts = await screen.findAllByText('The value is out of range.');
    expect(alerts.length).toBeGreaterThan(0);
    expect(input).toHaveAttribute('aria-invalid', 'true');
  });

  it('shows the error text and request ID for other failures', async () => {
    server.use(http.put(`${API}/admin/deadline`, () => problem(503, 'SERVICE_UNAVAILABLE', { requestId: 'req-9' })));
    const { user } = await open();
    const input = await screen.findByLabelText('Submission deadline');
    await waitFor(() => expect(input).not.toHaveValue(''));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByText(/Request ID: req-9/)).toBeInTheDocument();
  });
});
