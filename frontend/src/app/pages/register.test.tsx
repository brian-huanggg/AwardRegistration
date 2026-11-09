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

interface Seen {
  body: unknown;
  csrf: string | null;
  cookieSet: boolean;
}

/** Visitor on /register; returns what POST /auth/register received. */
async function open(locale: Locale = 'en') {
  clearCsrfToken();
  const seen: Seen[] = [];
  server.use(
    http.get(`${API}/auth/me`, () => problem(401, 'UNAUTHENTICATED')),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state: 'OPEN', serverTime: '2026-11-08T00:00:00.000Z' })),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
  );
  await initI18n(locale);
  const router = createMemoryRouter(routes, { initialEntries: ['/register'] });
  const view = render(<App router={router} client={new QueryClient()} />);
  await screen.findByRole('heading', { level: 1, name: locale === 'en' ? 'Create an account' : '建立帳號' });
  return { router, seen, user: userEvent.setup(), ...view };
}

function registerReturns(seen: Seen[], respond: () => Response | Promise<Response>) {
  server.use(
    http.post(`${API}/auth/register`, async ({ request }) => {
      seen.push({ body: await request.json(), csrf: request.headers.get('X-CSRF-Token'), cookieSet: false });
      return respond();
    }),
  );
}

async function fill(user: ReturnType<typeof userEvent.setup>, v: Partial<Record<'Employee ID' | 'Name' | 'Email' | 'Password' | 'Confirm password', string>>) {
  for (const [label, value] of Object.entries(v)) await user.type(screen.getByLabelText(label), value);
}

const VALID = { 'Employee ID': '089290', Name: '  Wang   Xiao Ming ', Email: ' Wang@Example.COM ', Password: 'Correct-Horse-9', 'Confirm password': 'Correct-Horse-9' };

describe('Register page: markup', () => {
  it('labels every field and sets autocomplete (WCAG 1.3.1, 1.3.5)', async () => {
    await open();
    const expected: [string, string][] = [
      ['Employee ID', 'username'],
      ['Name', 'name'],
      ['Email', 'email'],
      ['Password', 'new-password'],
      ['Confirm password', 'new-password'],
    ];
    for (const [label, ac] of expected) {
      const input = screen.getByLabelText(label);
      expect(input).toHaveAttribute('autocomplete', ac);
      expect(input).toBeRequired();
    }
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password');
    expect(screen.getByLabelText('Email')).toHaveAttribute('type', 'email');
  });

  it.each(['en', 'zh-TW'] as const)('has no axe violations (%s), also with errors shown', async (locale) => {
    const { container, user } = await open(locale);
    await screen.findByRole('heading', { level: 1 });
    expect((await axe(container)).violations).toEqual([]);
    await user.click(screen.getByRole('button', { name: locale === 'en' ? 'Create account' : '建立帳號' }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });
});

describe('Register page: pre-check and error summary', () => {
  it('blocks an empty submit, focuses the summary and sends no request', async () => {
    const { user } = await open();
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    const summary = await screen.findByRole('alert');
    await waitFor(() => expect(summary).toHaveFocus());
    const links = within(summary).getAllByRole('link');
    expect(links.map((l) => l.textContent)).toEqual([
      'Employee ID: This field is required.',
      'Name: This field is required.',
      'Email: This field is required.',
      'Password: This field is required.',
    ]);
    // each field points at its error text and is marked invalid
    const empNo = screen.getByLabelText('Employee ID');
    expect(empNo).toHaveAttribute('aria-invalid', 'true');
    expect(empNo.getAttribute('aria-describedby')).toContain('empNo-error');
    expect(document.getElementById('empNo-error')).toHaveTextContent('This field is required.');
  });

  it('a summary link moves focus to its field (keyboard: Enter)', async () => {
    const { user } = await open();
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    const summary = await screen.findByRole('alert');
    await waitFor(() => expect(summary).toHaveFocus());
    await user.tab();
    expect(within(summary).getAllByRole('link')[0]).toHaveFocus();
    await user.tab();
    await user.keyboard('{Enter}');
    expect(screen.getByLabelText('Name')).toHaveFocus();
  });

  it.each([
    ['12345', 'The format is not valid.'],
    ['12345a', 'The format is not valid.'],
    ['１２３４５６', 'The format is not valid.'],
  ])('employee ID %j', async (empNo, msg) => {
    const { user } = await open();
    await fill(user, { ...VALID, 'Employee ID': empNo });
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByText(`Employee ID: ${msg}`)).toBeInTheDocument();
  });

  it('email with a space is rejected before any request (PRD §6.3)', async () => {
    const { user } = await open();
    await fill(user, { ...VALID, Email: 'a b@x.com' });
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByText('Email: The format is not valid.')).toBeInTheDocument();
  });

  it('name with a forbidden character and a mismatching confirmation', async () => {
    const { user } = await open();
    await fill(user, { ...VALID, Name: 'a/b', 'Confirm password': 'other' });
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByText('Name: The name contains characters that are not allowed.')).toBeInTheDocument();
    expect(screen.getByText('Confirm password: The passwords do not match.')).toBeInTheDocument();
  });
});

describe('Register page: live password checklist (F1b-01)', () => {
  const status = (rule: string) => document.querySelector(`li[data-rule="${rule}"]`)?.getAttribute('data-status');

  it('the password field is described by the checklist, which follows typing and the employee ID', async () => {
    const { user } = await open();
    const password = screen.getByLabelText('Password');
    expect(password.getAttribute('aria-describedby')).toContain('password-rules');
    expect(document.getElementById('password-rules')).toHaveTextContent('Your password must:');
    expect(status('LENGTH')).toBe('idle');

    await fill(user, { 'Employee ID': '089290', Password: 'Password089290' });
    await waitFor(() => expect(status('COMMON_PASSWORD')).toBe('met'));
    expect(status('LENGTH')).toBe('met');
    expect(status('CONTAINS_EMP_NO')).toBe('unmet');
  });

  it('the Top-10k blocklist (lazy chunk) marks a common password', async () => {
    const { user } = await open();
    await fill(user, { Password: 'password' });
    await waitFor(() => expect(status('COMMON_PASSWORD')).toBe('unmet'));
  });

  it('does not block submit: the server stays the authority (422 PASSWORD_POLICY)', async () => {
    const { seen, user } = await open();
    registerReturns(seen, () => problem(422, 'PASSWORD_POLICY', { errors: [{ field: 'password', code: 'PASSWORD_POLICY', rule: 'LENGTH' }] }));
    await fill(user, { ...VALID, Password: 'short', 'Confirm password': 'short' });
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByText('Password: The password does not meet the requirements.')).toBeInTheDocument();
    expect(seen).toHaveLength(1);
  });
});

describe('Register page: submit', () => {
  it('sends the cleaned values with the CSRF token, then goes to /login with a notice and no login', async () => {
    const { user, seen, router } = await open();
    registerReturns(seen, () => HttpResponse.json({ empNo: '089290', name: 'Wang Xiao Ming', email: 'wang@example.com' }, { status: 201 }));
    let loginCalls = 0;
    server.use(http.post(`${API}/auth/login`, () => (loginCalls++, problem(500, 'INTERNAL_ERROR'))));
    await fill(user, VALID);
    await user.keyboard('{Enter}'); // submit from the keyboard
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(seen).toEqual([
      { body: { empNo: '089290', name: 'Wang Xiao Ming', email: 'wang@example.com', password: 'Correct-Horse-9' }, csrf: 'tok', cookieSet: false },
    ]);
    expect(router.state.location.search).toBe('');
    expect(router.state.location.state).toEqual({ notice: 'registered' });
    expect(await screen.findByRole('status', { name: '' })).toBeDefined();
    expect(screen.getByText('Your account was created. Sign in to continue.')).toBeInTheDocument();
    expect(loginCalls).toBe(0);
  });

  it('disables the button while the request is pending (no double submit)', async () => {
    const { user, seen, router } = await open();
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    registerReturns(seen, async () => {
      await gate;
      return HttpResponse.json({ empNo: '089290', name: 'W', email: 'w@x.io' }, { status: 201 });
    });
    await fill(user, VALID);
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    const busy = await screen.findByRole('button', { name: 'Creating account…' });
    expect(busy).toBeDisabled();
    release();
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(seen).toHaveLength(1);
  });

  it.each([
    ['EMP_NO_TAKEN', 'Employee ID', 'Employee ID: This employee ID is already registered.'],
    ['EMAIL_TAKEN', 'Email', 'Email: This email address is already registered.'],
  ] as const)('409 %s is shown on its field and the user stays on the form', async (code, label, summaryText) => {
    const { user, seen, router } = await open();
    registerReturns(seen, () => problem(409, code));
    await fill(user, VALID);
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByRole('link', { name: summaryText })).toBeInTheDocument();
    expect(screen.getByLabelText(label)).toHaveAttribute('aria-invalid', 'true');
    expect(router.state.location.pathname).toBe('/register');
    expect(screen.getByLabelText('Password')).toHaveValue('Correct-Horse-9'); // nothing the user typed is lost
  });

  it('422 field errors map to their fields; PASSWORD_POLICY shows the policy message', async () => {
    const { user, seen } = await open();
    registerReturns(seen, () =>
      problem(422, 'PASSWORD_POLICY', {
        errors: [
          { field: 'name', code: 'OUT_OF_RANGE' },
          { field: 'password', code: 'LENGTH' },
          { field: 'password', code: 'COMPLEXITY' },
        ],
      }),
    );
    await fill(user, VALID);
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByRole('link', { name: 'Name: The value is out of range.' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Password: The password does not meet the requirements.' })).toBeInTheDocument();
    expect(screen.getByLabelText('Name')).toHaveAttribute('aria-invalid', 'true');
  });

  it('a failure without a field (503) is a form-level message with the request ID', async () => {
    const { user, seen, router } = await open();
    registerReturns(seen, () => problem(503, 'SERVICE_UNAVAILABLE'));
    await fill(user, VALID);
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    const summary = await screen.findByRole('alert');
    expect(summary).toHaveTextContent('The service is temporarily unavailable. Please try again later.');
    expect(summary).toHaveTextContent('Request ID: 6f1c2c1e-0000-4000-8000-000000000000');
    expect(within(summary).queryAllByRole('link')).toHaveLength(0);
    expect(router.state.location.pathname).toBe('/register');
    expect(screen.getByRole('button', { name: 'Create account' })).toBeEnabled();
  });

  it('retries once after CSRF_INVALID (shared client behaviour) and still succeeds', async () => {
    const { user, router } = await open();
    let n = 0;
    server.use(http.post(`${API}/auth/register`, () => (n++ === 0 ? problem(403, 'CSRF_INVALID') : HttpResponse.json({ empNo: '089290', name: 'W', email: 'w@x.io' }, { status: 201 }))));
    await fill(user, VALID);
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(n).toBe(2);
  });
});
