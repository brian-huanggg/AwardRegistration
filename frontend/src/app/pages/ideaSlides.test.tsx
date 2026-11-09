import { QueryClient } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter } from 'react-router';
import { axe } from 'vitest-axe';
import { clearCsrfToken } from '@/shared/api';
import { problem } from '@/shared/api/mocks/problem';
import { server } from '@/shared/api/mocks/server';
import { readFormData } from '@/shared/api/mocks/formData';
import { initI18n } from '@/shared/i18n';
import { App } from '../App';
import { routes } from '../router';

const API = 'http://localhost:3000/api/v1';
const ID = '0193a1b2-0000-7000-8000-000000000001';
const LEADER = { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' };
const MEMBER = { empNo: '089291', name: '李小華', email: 'lee@example.com', role: 'USER' };
const ADMIN = { empNo: '000001', name: 'Admin', email: null, role: 'ADMIN' };
const BASE = 'I_BSID_王小明_089290_成立供應商永續學院';

const detail = (over: Record<string, unknown> = {}) => ({
  id: ID,
  name: '成立供應商永續學院',
  category: 'COOL_IDEA',
  org: 'BSID',
  direction: 'TALENT_DEVELOPMENT',
  summary: 'line one',
  leader: { empNo: '089290', name: '王小明' },
  members: [],
  pendingRequests: [],
  slides: { ext: 'pdf', size: 2048, sha256: 'x', updatedAt: '2026-11-02T03:00:00.000Z', downloadFilename: `${BASE}.pdf` },
  expectedFilenameBase: BASE,
  createdAt: '2026-11-02T03:00:00.000Z',
  updatedAt: '2026-11-02T03:00:00.000Z',
  version: 1,
  ...over,
});

const pdf = (name = `${BASE}.pdf`) => new File(['%PDF-1.4\nbody'], name, { type: 'application/pdf' });

async function open({ me = LEADER as { empNo: string; name: string; email: string | null; role: string }, state = 'OPEN' } = {}) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state, serverTime: '2026-11-08T00:00:00.000Z' })),
    http.get(`${API}/auth/csrf`, () => HttpResponse.json({ token: 'tok' })),
  );
  await initI18n('en');
  const router = createMemoryRouter(routes, { initialEntries: [`/ideas/${ID}`] });
  const view = render(<App router={router} client={new QueryClient()} />);
  return { router, user: userEvent.setup(), ...view };
}

const idea = (over: Record<string, unknown> = {}) => {
  let gets = 0;
  server.use(http.get(`${API}/ideas/${ID}`, () => (gets++, HttpResponse.json(detail(over)))));
  return () => gets;
};

describe('Replace slides', () => {
  it('is offered to the Leader and an Admin, not to a Member', async () => {
    idea();
    const a = await open();
    expect(await screen.findByRole('button', { name: 'Replace slides' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Delete Idea' })).toBeEnabled();
    a.unmount();
    idea();
    const b = await open({ me: ADMIN });
    expect(await screen.findByRole('button', { name: 'Replace slides' })).toBeEnabled();
    b.unmount();
    idea();
    await open({ me: MEMBER });
    await screen.findByRole('button', { name: 'Download' });
    expect(screen.queryByRole('button', { name: 'Replace slides' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Delete Idea' })).not.toBeInTheDocument();
  });

  it('disables both for the Leader when submissions are closed, but not for an Admin', async () => {
    idea();
    const a = await open({ state: 'CLOSED' });
    await screen.findByRole('button', { name: 'Download' });
    await waitFor(() => expect(screen.getByRole('button', { name: 'Replace slides' })).toBeDisabled());
    expect(screen.getByRole('button', { name: 'Delete Idea' })).toBeDisabled();
    a.unmount();
    idea();
    await open({ me: ADMIN, state: 'CLOSED' });
    expect(await screen.findByRole('button', { name: 'Replace slides' })).toBeEnabled();
  });

  it('pre-checks the file name against the current expected name and sends nothing', async () => {
    idea();
    let puts = 0;
    server.use(http.put(`${API}/ideas/${ID}/slides`, () => (puts++, HttpResponse.json(detail()))));
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Replace slides' }));
    await user.upload(screen.getByLabelText('Slide file'), pdf('wrong.pdf'));
    await user.click(screen.getByRole('button', { name: 'Replace' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(BASE);
    expect(screen.getByLabelText('Slide file')).toHaveAttribute('aria-invalid', 'true');
    expect(puts).toBe(0);
  });

  it('sends one multipart part and shows the new detail without another GET', async () => {
    const gets = idea();
    let sent: { csrf: string | null; file: FormDataEntryValue | null } | null = null;
    server.use(
      http.put(`${API}/ideas/${ID}/slides`, async ({ request }) => {
        const form = await readFormData(request);
        sent = { csrf: request.headers.get('X-CSRF-Token'), file: form.get('slides') };
        return HttpResponse.json(detail({ version: 2, slides: { ...detail().slides, size: 4096, updatedAt: '2026-11-08T03:00:00.000Z' } }));
      }),
    );
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Replace slides' }));
    await user.upload(screen.getByLabelText('Slide file'), pdf());
    await user.click(screen.getByRole('button', { name: 'Replace' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(sent!.csrf).toBe('tok');
    expect((sent!.file as File).name).toBe(`${BASE}.pdf`);
    expect(await screen.findByText(/4 KB/)).toBeInTheDocument();
    expect(gets()).toBe(1);
  });

  it('maps a server FILENAME_MISMATCH to the file field', async () => {
    idea();
    server.use(http.put(`${API}/ideas/${ID}/slides`, () => problem(422, 'FILENAME_MISMATCH')));
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Replace slides' }));
    await user.upload(screen.getByLabelText('Slide file'), pdf());
    await user.click(screen.getByRole('button', { name: 'Replace' }));
    await waitFor(() => expect(screen.getByLabelText('Slide file')).toHaveAttribute('aria-invalid', 'true'));
  });

  it('says the result is uncertain on 503 SERVICE_UNAVAILABLE and refreshes the detail', async () => {
    const gets = idea();
    server.use(http.put(`${API}/ideas/${ID}/slides`, () => problem(503, 'SERVICE_UNAVAILABLE')));
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Replace slides' }));
    await user.upload(screen.getByLabelText('Slide file'), pdf());
    await user.click(screen.getByRole('button', { name: 'Replace' }));
    expect(await screen.findByText(/result is uncertain/)).toBeInTheDocument();
    await waitFor(() => expect(gets()).toBe(2));
    expect(screen.getByRole('button', { name: 'Replace' })).toBeEnabled();
  });

  it('closes with Esc and returns focus to the trigger; passes axe', async () => {
    idea();
    const { user } = await open();
    const trigger = await screen.findByRole('button', { name: 'Replace slides' });
    await user.click(trigger);
    expect((await axe(screen.getByRole('dialog'))).violations).toEqual([]);
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });
});

describe('Delete Idea', () => {
  it('asks first; Cancel and Esc send nothing and return focus to the trigger', async () => {
    idea();
    let dels = 0;
    server.use(http.delete(`${API}/ideas/${ID}`, () => (dels++, new HttpResponse(null, { status: 204 }))));
    const { user } = await open();
    const trigger = await screen.findByRole('button', { name: 'Delete Idea' });
    await user.click(trigger);
    const dialog = screen.getByRole('alertdialog');
    expect(dialog).toHaveTextContent('cannot be undone');
    expect((await axe(dialog)).violations).toEqual([]);
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
    await user.click(trigger);
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(dels).toBe(0);
  });

  it('confirms with the keyboard, sends DELETE and goes to the list', async () => {
    idea();
    let csrf: string | null = null;
    server.use(
      http.delete(`${API}/ideas/${ID}`, ({ request }) => ((csrf = request.headers.get('X-CSRF-Token')), new HttpResponse(null, { status: 204 }))),
      http.get(`${API}/ideas`, () => HttpResponse.json({ items: [], total: 0, page: 1, pageSize: 20 })),
    );
    const { router, user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Delete Idea' }));
    screen.getByRole('button', { name: 'Delete Idea' }).focus();
    await user.keyboard('{Enter}');
    await waitFor(() => expect(router.state.location.pathname).toBe('/ideas'));
    expect(csrf).toBe('tok');
  });

  it('keeps the dialog open and shows the error when the delete fails', async () => {
    idea();
    server.use(http.delete(`${API}/ideas/${ID}`, () => problem(503, 'SERVICE_UNAVAILABLE')));
    const { router, user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Delete Idea' }));
    await user.click(screen.getByRole('button', { name: 'Delete Idea' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('temporarily unavailable');
    expect(router.state.location.pathname).toBe(`/ideas/${ID}`);
    expect(screen.getByRole('button', { name: 'Delete Idea' })).toBeEnabled();
  });

  it('treats 404 as already deleted', async () => {
    idea();
    server.use(
      http.delete(`${API}/ideas/${ID}`, () => problem(404, 'NOT_FOUND')),
      http.get(`${API}/ideas`, () => HttpResponse.json({ items: [], total: 0, page: 1, pageSize: 20 })),
    );
    const { router, user } = await open({ me: ADMIN });
    await user.click(await screen.findByRole('button', { name: 'Delete Idea' }));
    await user.click(screen.getByRole('button', { name: 'Delete Idea' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/ideas'));
  });
});
