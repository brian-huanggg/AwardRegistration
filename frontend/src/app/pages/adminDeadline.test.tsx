import { QueryClient } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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

// The admin page is lazy-loaded; the first import in a run can take longer than the default 1 s.
const dateInput = () => screen.findByLabelText<HTMLInputElement>('Date', {}, { timeout: 5000 });
const timeInput = () => screen.getByLabelText<HTMLInputElement>('Time');
const pill = () => screen.getByRole('region', { name: 'Submission status' });
const current = () => screen.getByRole('heading', { level: 3, name: 'Current deadline' }).parentElement!;

describe('Admin deadline page', () => {
  it('shows the current deadline, prefills Date + Time, keeps the shell pill and passes axe', async () => {
    const { container } = await open();
    const date = await dateInput();
    expect(screen.getByRole('heading', { level: 1, name: 'Deadline' })).toBeInTheDocument();
    await waitFor(() => expect(date.value).toMatch(/^2026-12-\d{2}$/));
    expect(timeInput().value).toMatch(/^\d{2}:\d{2}:\d{2}$/);
    // The page-level banner is gone, so the shell pill shows here too (one region, landmark-unique).
    await waitFor(() => expect(pill()).toHaveAttribute('data-state', 'OPEN'));
    expect(screen.getAllByRole('region', { name: 'Submission status' })).toHaveLength(1);
    expect(within(current()).getByText('2026-12-01T02:30:00.000Z')).toBeInTheDocument();
    expect(within(current()).getByText('OPEN')).toBeInTheDocument();
    // Read-only browser zone, not a select (UI-07 Design vs contract).
    expect(screen.getByLabelText('Time zone')).toHaveTextContent(Intl.DateTimeFormat().resolvedOptions().timeZone);
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'How submission state works' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /clear/i })).not.toBeInTheDocument();
    expect(await screen.findByText(/^Users will see: Submissions open · closes in 23d \d{2}h$/)).toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('a User sees the 404 page', async () => {
    await open(USER);
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
    expect(screen.queryByLabelText('Date')).not.toBeInTheDocument();
  });

  it('shows the "not set" variant with empty inputs when no deadline exists', async () => {
    await open(ADMIN, body('NOT_OPEN', null));
    const date = await dateInput();
    expect(await within(current()).findByText('Not set')).toBeInTheDocument();
    expect(within(current()).getByText('NOT OPEN')).toBeInTheDocument();
    expect(date).toHaveValue('');
    expect(timeInput()).toHaveValue('');
  });

  it('previews OPEN for a future instant and CLOSED for a past one, using server time', async () => {
    await open();
    const date = await dateInput();
    await waitFor(() => expect(date.value).not.toBe(''));
    const live = (await screen.findByText(/^Users will see:/)).closest('[aria-live]');
    expect(live).toHaveAttribute('aria-live', 'polite');
    // serverTime is 2026-11-08; a date before it closes submissions even if the client clock says otherwise.
    fireEvent.change(date, { target: { value: '2026-11-07' } });
    expect(await screen.findByText('Users will see: Submissions closed')).toBeInTheDocument();
    expect(screen.getByText(/A time in the past closes submissions immediately/).parentElement).toHaveAttribute('data-prominent', 'true');
    fireEvent.change(date, { target: { value: '2026-11-10' } });
    expect(await screen.findByText(/^Users will see: Submissions open · closes in [12]d \d{2}h$/)).toBeInTheDocument();
    expect(screen.getByText(/A time in the past closes submissions immediately/).parentElement).not.toHaveAttribute('data-prominent');
  });

  it('Reset restores the inputs to the current deadline', async () => {
    const { user } = await open();
    const date = await dateInput();
    await waitFor(() => expect(date.value).not.toBe(''));
    const before = { date: date.value, time: timeInput().value };
    fireEvent.change(date, { target: { value: '2027-01-15' } });
    fireEvent.change(timeInput(), { target: { value: '08:15:00' } });
    expect(date).toHaveValue('2027-01-15');
    const reset = screen.getByRole('button', { name: 'Reset' });
    expect(reset).toHaveAttribute('type', 'button');
    await user.click(reset);
    expect(date).toHaveValue(before.date);
    expect(timeInput()).toHaveValue(before.time);
  });

  it('PUTs ISO 8601 with offset and CSRF, then refreshes the pill and current block without another GET', async () => {
    let sent: { body: unknown; csrf: string | null } | null = null;
    server.use(
      http.put(`${API}/admin/deadline`, async ({ request }) => {
        sent = { body: await request.json(), csrf: request.headers.get('X-CSRF-Token') };
        return HttpResponse.json(body('OPEN', '2026-12-31T15:00:00.000Z'));
      }),
    );
    const { user, container } = await open(ADMIN, body('NOT_OPEN', null));
    const date = await dateInput();
    await waitFor(() => expect(pill()).toHaveAttribute('data-state', 'NOT_OPEN'));
    fireEvent.change(date, { target: { value: '2026-12-31' } });
    fireEvent.change(timeInput(), { target: { value: '23:00:00' } });
    await user.click(screen.getByRole('button', { name: 'Save deadline' }));
    expect(await screen.findByText('The deadline was saved.')).toBeInTheDocument();
    expect(sent!.csrf).toBe('tok');
    expect((sent!.body as { deadline: string }).deadline).toMatch(/^2026-12-31T23:00:00[+-]\d{2}:\d{2}$/);
    await waitFor(() => expect(pill()).toHaveAttribute('data-state', 'OPEN'));
    expect(within(pill()).getByText('Submissions are open.')).toBeInTheDocument();
    expect(within(current()).getByText('2026-12-31T15:00:00.000Z')).toBeInTheDocument();
    expect(within(current()).getByText('OPEN')).toBeInTheDocument();
    expect(gets).toBe(1);
    expect((await axe(container)).violations).toEqual([]);
  });

  it('rejects an incomplete date and time on the client', async () => {
    let puts = 0;
    server.use(http.put(`${API}/admin/deadline`, () => (puts++, HttpResponse.json(body()))));
    const { user } = await open(ADMIN, body('NOT_OPEN', null));
    const date = await dateInput();
    fireEvent.change(date, { target: { value: '2026-12-31' } });
    await user.click(screen.getByRole('button', { name: 'Save deadline' }));
    expect((await screen.findAllByText('The format is not valid.')).length).toBeGreaterThan(0);
    expect(date).toHaveAttribute('aria-invalid', 'true');
    expect(timeInput()).toHaveAttribute('aria-invalid', 'true');
    expect(puts).toBe(0);
  });

  it('maps a 422 OUT_OF_RANGE to the fields, focuses the summary and passes axe', async () => {
    server.use(
      http.put(`${API}/admin/deadline`, () =>
        problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'deadline', code: 'OUT_OF_RANGE' }] }),
      ),
    );
    const { user, container } = await open();
    const date = await dateInput();
    await waitFor(() => expect(date).not.toHaveValue(''));
    await user.click(screen.getByRole('button', { name: 'Save deadline' }));
    const alerts = await screen.findAllByText('The value is out of range.');
    expect(alerts.length).toBeGreaterThan(0);
    expect(date).toHaveAttribute('aria-invalid', 'true');
    expect(timeInput()).toHaveAttribute('aria-invalid', 'true');
    await waitFor(() => expect(screen.getByRole('alert', { name: 'The deadline was not saved' })).toHaveFocus());
    expect((await axe(container)).violations).toEqual([]);
  });

  it('shows the error text and request ID for other failures', async () => {
    server.use(http.put(`${API}/admin/deadline`, () => problem(503, 'SERVICE_UNAVAILABLE', { requestId: 'req-9' })));
    const { user } = await open();
    const date = await dateInput();
    await waitFor(() => expect(date).not.toHaveValue(''));
    await user.click(screen.getByRole('button', { name: 'Save deadline' }));
    expect(await screen.findByText(/Request ID: req-9/)).toBeInTheDocument();
  });
});
