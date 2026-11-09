import { QueryClient } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter } from 'react-router';
import { axe } from 'vitest-axe';
import { clearCsrfToken } from '@/shared/api';
import { readFormData } from '@/shared/api/mocks/formData';
import { problem } from '@/shared/api/mocks/problem';
import { server } from '@/shared/api/mocks/server';
import { initI18n, type Locale } from '@/shared/i18n';
import { App } from '../App';
import { routes } from '../router';

const API = 'http://localhost:3000/api/v1';
const ME = { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' };
const BASE = 'I_BSID_王小明_089290_成立供應商永續學院';
const PDF = '%PDF-1.7\n';
const ID = '0193a1b2-0000-7000-8000-000000000001';

type State = 'OPEN' | 'CLOSED' | 'NOT_OPEN';

interface Sent {
  fields: Record<string, string>;
  file: { name: string; size: number } | null;
  csrf: string | null;
}

async function open({ locale = 'en', state = 'OPEN' as State } = {}) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(ME)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state, serverTime: '2026-11-08T00:00:00.000Z' })),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
  );
  await initI18n(locale as Locale);
  const router = createMemoryRouter(routes, { initialEntries: ['/ideas/new'] });
  const client = new QueryClient();
  const view = render(<App router={router} client={client} />);
  await screen.findByRole('heading', { level: 1, name: locale === 'en' ? 'Create an Idea' : '建立提案' });
  return { router, client, user: userEvent.setup(), ...view };
}

function createReturns(respond: () => Response | Promise<Response>) {
  const sent: Sent[] = [];
  server.use(
    http.post(`${API}/ideas`, async ({ request }) => {
      const form = await readFormData(request);
      const fields: Record<string, string> = {};
      let file: Sent['file'] = null;
      for (const [k, v] of form.entries()) {
        if (typeof v === 'string') fields[k] = v;
        else file = { name: v.name, size: v.size };
      }
      sent.push({ fields, file, csrf: request.headers.get('X-CSRF-Token') });
      return respond();
    }),
  );
  return sent;
}

type User = ReturnType<typeof userEvent.setup>;

async function fillText(user: User, v: { name?: string; summary?: string } = {}) {
  await user.selectOptions(screen.getByLabelText('Category'), 'COOL_IDEA');
  await user.selectOptions(screen.getByLabelText('Organization'), 'BSID');
  await user.selectOptions(screen.getByLabelText('Direction'), 'RESPONSIBLE_SUPPLY_CHAIN');
  await user.type(screen.getByLabelText('Idea name'), v.name ?? '  成立供應商永續學院 ');
  await user.type(screen.getByLabelText('Summary'), v.summary ?? 'Train suppliers.');
}

const pick = (user: User, name: string, content: string = PDF) => user.upload(screen.getByLabelText('Slide file'), new File([content], name));

const submit = (user: User) => user.click(screen.getByRole('button', { name: 'Submit Idea' }));

describe('Create Idea page: markup and accessibility', () => {
  it('labels and requires every field; the Leader comes from the session', async () => {
    await open();
    for (const label of ['Category', 'Organization', 'Direction', 'Idea name', 'Summary', 'Slide file']) {
      expect(screen.getByLabelText(label)).toBeRequired();
    }
    expect(screen.getByText('王小明 (089290)')).toBeInTheDocument();
    expect(screen.getByLabelText('Slide file')).toHaveAttribute('type', 'file');
    expect(screen.getByLabelText('Idea name')).not.toHaveAttribute('maxlength'); // counted in code points, not UTF-16
  });

  it('shows the zh-TW working-draft labels (OI-17)', async () => {
    await open({ locale: 'zh-TW' });
    const options = within(screen.getByLabelText('類別')).getAllByRole('option').map((o) => o.textContent);
    expect(options).toEqual(['請選擇…', '酷點子獎', '卓越影響獎']);
    expect(within(screen.getByLabelText('方向')).getByRole('option', { name: '關懷弱勢' })).toBeInTheDocument();
  });

  it.each(['en', 'zh-TW'] as const)('has no axe violations (%s), with the expected name and with errors shown', async (locale) => {
    const { container, user } = await open({ locale });
    expect((await axe(container)).violations).toEqual([]);
    if (locale === 'en') {
      await fillText(user);
      expect(await screen.findByText(`${BASE}.pdf`)).toBeInTheDocument();
      expect((await axe(container)).violations).toEqual([]);
    }
    await user.click(screen.getByRole('button', { name: locale === 'en' ? 'Submit Idea' : '送出提案' }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });
});

describe('Create Idea page: code-point counters', () => {
  it('count the cleaned value in code points and are linked to their field', async () => {
    const { user } = await open();
    const name = screen.getByLabelText('Idea name');
    expect(name.getAttribute('aria-describedby')).toContain('name-count');
    expect(document.getElementById('name-count')).toHaveTextContent('0 / 75 characters');
    await user.type(name, '  😀😀  a\tb ');
    expect(document.getElementById('name-count')).toHaveTextContent('6 / 75 characters'); // "😀😀 a b"
    await user.type(screen.getByLabelText('Summary'), '創意\n\n😀');
    expect(document.getElementById('summary-count')).toHaveTextContent('5 / 300 characters');
  });

  it('76 code points are rejected before any request (OUT_OF_RANGE)', async () => {
    const { user } = await open();
    const sent = createReturns(() => HttpResponse.json({}, { status: 201 }));
    await fillText(user, { name: '😀'.repeat(76) });
    expect(document.getElementById('name-count')).toHaveTextContent('76 / 75 characters');
    await submit(user);
    expect(await screen.findByRole('link', { name: 'Idea name: The value is out of range.' })).toBeInTheDocument();
    expect(sent).toHaveLength(0);
  });
});

describe('Create Idea page: expected filename (PRD §6.8, OI-21)', () => {
  it('appears once category, org and name are given, for both extensions, from the cleaned name', async () => {
    const { user } = await open();
    expect(screen.getByText(/to see the expected file name/)).toBeInTheDocument();
    await fillText(user);
    expect(screen.getByText(`${BASE}.pdf`)).toBeInTheDocument();
    expect(screen.getByText(`${BASE}.pptx`)).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('Category'), 'OUTSTANDING_IMPACT');
    expect(screen.getByText(`O${BASE.slice(1)}.pdf`)).toBeInTheDocument();
  });

  it('the copy buttons are named and copy the full name; the result is announced', async () => {
    const { user } = await open();
    await fillText(user);
    await user.click(screen.getByRole('button', { name: 'Copy the PPTX file name' }));
    expect(await navigator.clipboard.readText()).toBe(`${BASE}.pptx`);
    expect(screen.getByText('File name copied.')).toHaveAttribute('role', 'status');
  });

  it('a name whose expected filename exceeds 255 UTF-8 bytes is FILENAME_TOO_LONG on the name', async () => {
    const { user } = await open();
    const sent = createReturns(() => HttpResponse.json({}, { status: 201 }));
    await fillText(user, { name: '😀'.repeat(75) }); // 75 code points, 300 UTF-8 bytes in the name alone
    await pick(user, 'x.pdf');
    await submit(user);
    expect(await screen.findByRole('link', { name: 'Idea name: The file name is too long.' })).toBeInTheDocument();
    expect(sent).toHaveLength(0);
  });
});

describe('Create Idea page: pre-check', () => {
  it('an empty submit lists every field, focuses the summary and sends nothing', async () => {
    const { user } = await open();
    const sent = createReturns(() => HttpResponse.json({}, { status: 201 }));
    await submit(user);
    const summary = await screen.findByRole('alert');
    await waitFor(() => expect(summary).toHaveFocus());
    expect(within(summary).getAllByRole('link').map((l) => l.textContent)).toEqual([
      'Category: This field is required.',
      'Organization: This field is required.',
      'Direction: This field is required.',
      'Idea name: This field is required.',
      'Summary: This field is required.',
      'Slide file: This field is required.',
    ]);
    const slides = screen.getByLabelText('Slide file');
    expect(slides).toHaveAttribute('aria-invalid', 'true');
    expect(slides.getAttribute('aria-describedby')).toContain('slides-error');
    await user.click(within(summary).getByRole('link', { name: /^Summary/ }));
    expect(screen.getByLabelText('Summary')).toHaveFocus();
    expect(sent).toHaveLength(0);
  });

  it('a name with a forbidden character: text errors only, the file is not checked yet', async () => {
    const { user } = await open();
    await fillText(user, { name: 'a/b' });
    await pick(user, 'wrong.pdf');
    await submit(user);
    const summary = await screen.findByRole('alert');
    expect(within(summary).getAllByRole('link').map((l) => l.textContent)).toEqual(['Idea name: The name contains characters that are not allowed.']);
  });

  it.each([
    ['a wrong name', 'slides.pdf', PDF, `Slide file: The file name does not match the expected name. Expected: ${BASE}.pdf`],
    ['an empty file', `${BASE}.pdf`, '', 'Slide file: The file is empty.'],
    ['PDF bytes named .pptx', `${BASE}.pptx`, PDF, 'Slide file: This file type is not supported.'],
  ])('%s is rejected before upload', async (_n, name, content, text) => {
    const { user } = await open();
    const sent = createReturns(() => HttpResponse.json({}, { status: 201 }));
    await fillText(user);
    await pick(user, name, content);
    await submit(user);
    expect(await screen.findByRole('link', { name: text })).toBeInTheDocument();
    expect(sent).toHaveLength(0);
  });
});

describe('Create Idea page: submit', () => {
  it('sends one multipart request with the cleaned fields, the file and the CSRF token, then opens the new Idea', async () => {
    const { user, router, client } = await open();
    const detail = { id: ID, name: '成立供應商永續學院', expectedFilenameBase: BASE };
    const sent = createReturns(() => HttpResponse.json(detail, { status: 201, headers: { Location: `/api/v1/ideas/${ID}` } }));
    await fillText(user, { summary: ' Line 1\r\nLine 2 ' });
    await pick(user, `${BASE}.pdf`);
    await submit(user);
    await waitFor(() => expect(router.state.location.pathname).toBe(`/ideas/${ID}`));
    expect(sent).toEqual([
      {
        fields: { category: 'COOL_IDEA', org: 'BSID', direction: 'RESPONSIBLE_SUPPLY_CHAIN', name: '成立供應商永續學院', summary: 'Line 1\nLine 2' },
        file: { name: `${BASE}.pdf`, size: PDF.length },
        csrf: 'tok',
      },
    ]);
    expect(client.getQueryData(['idea', ID])).toEqual(detail);
  });

  it('the button is disabled while the request is pending (no double submit)', async () => {
    const { user, router } = await open();
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    const sent = createReturns(async () => {
      await gate;
      return HttpResponse.json({ id: ID }, { status: 201 });
    });
    await fillText(user);
    await pick(user, `${BASE}.pdf`);
    await submit(user);
    expect(await screen.findByRole('button', { name: 'Submitting…' })).toBeDisabled();
    release();
    await waitFor(() => expect(router.state.location.pathname).toBe(`/ideas/${ID}`));
    expect(sent).toHaveLength(1);
  });

  it('outside the submission window the button is disabled with a reason (not a guard)', async () => {
    await open({ state: 'CLOSED' });
    expect(await screen.findByText('Submissions are not open, so the Idea cannot be submitted now.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Submit Idea' })).toBeDisabled();
    expect(screen.getByLabelText('Idea name')).toBeEnabled();
  });
});

describe('Create Idea page: server answers', () => {
  async function submitValid(respond: () => Response) {
    const ctx = await open();
    const sent = createReturns(respond);
    await fillText(ctx.user);
    await pick(ctx.user, `${BASE}.pdf`);
    await submit(ctx.user);
    return { ...ctx, sent };
  }

  it('422 FILENAME_MISMATCH shows the server’s expected name on the file field', async () => {
    const expected = 'I_BSID_王小明_089290_Other.pdf';
    await submitValid(() => problem(422, 'FILENAME_MISMATCH', { errors: [{ field: 'slides', code: 'FILENAME_MISMATCH', expected }] }));
    expect(await screen.findByRole('link', { name: `Slide file: The file name does not match the expected name. Expected: ${expected}` })).toBeInTheDocument();
  });

  it('409 IDEA_DUPLICATE belongs to the name; nothing typed is lost', async () => {
    const { router } = await submitValid(() => problem(409, 'IDEA_DUPLICATE'));
    expect(await screen.findByRole('link', { name: 'Idea name: You already have an Idea with this name.' })).toBeInTheDocument();
    expect(screen.getByLabelText('Idea name')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByLabelText('Summary')).toHaveValue('Train suppliers.');
    expect(router.state.location.pathname).toBe('/ideas/new');
  });

  it('422 text errors map to their fields', async () => {
    await submitValid(() =>
      problem(422, 'VALIDATION_FAILED', {
        errors: [
          { field: 'direction', code: 'INVALID_ENUM' },
          { field: 'summary', code: 'OUT_OF_RANGE' },
        ],
      }),
    );
    expect(await screen.findByRole('link', { name: 'Direction: Please choose one of the listed options.' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Summary: The value is out of range.' })).toBeInTheDocument();
  });

  it.each([
    ['413 from the ingress (not JSON)', () => new HttpResponse('<html>413</html>', { status: 413, headers: { 'Content-Type': 'text/html' } }), 'Slide file: The file is too large (limit 5 MiB).'],
    ['415', () => problem(415, 'UNSUPPORTED_FILE_TYPE'), 'Slide file: This file type is not supported.'],
    ['422 EMPTY_FILE', () => problem(422, 'EMPTY_FILE'), 'Slide file: The file is empty.'],
  ])('%s is shown on the file field', async (_n, respond, text) => {
    await submitValid(respond);
    expect(await screen.findByRole('link', { name: text })).toBeInTheDocument();
  });

  it('403 SUBMISSION_CLOSED is a form message and refetches the deadline, which disables the button', async () => {
    await submitValid(() => {
      server.use(http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state: 'CLOSED', serverTime: '2026-11-08T00:00:00.000Z' })));
      return problem(403, 'SUBMISSION_CLOSED');
    });
    const summary = await screen.findByRole('alert');
    expect(summary).toHaveTextContent('Submissions are closed.');
    expect(summary).toHaveTextContent('Request ID: 6f1c2c1e-0000-4000-8000-000000000000');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Submit Idea' })).toBeDisabled());
    expect(screen.getByText('Submissions are not open, so the Idea cannot be submitted now.')).toBeInTheDocument();
  });
});
