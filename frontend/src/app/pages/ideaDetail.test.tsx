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
const USER = { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' };
const ADMIN = { empNo: '000001', name: 'Admin', email: null, role: 'ADMIN' };

const detail = (over: Record<string, unknown> = {}) => ({
  id: ID,
  name: '成立供應商永續學院',
  category: 'COOL_IDEA',
  org: 'BSID',
  direction: 'TALENT_DEVELOPMENT',
  summary: 'line one\nline two',
  leader: { empNo: '089290', name: '王小明' },
  members: [{ empNo: '089291', name: '李小華', joinedAt: '2026-11-03T03:00:00.000Z' }],
  pendingRequests: [{ empNo: '089292', name: '陳大文', requestedAt: '2026-11-04T03:00:00.000Z' }],
  slides: { ext: 'pdf', size: 2048, sha256: 'x', updatedAt: '2026-11-02T03:00:00.000Z', downloadFilename: 'I_BSID_王小明_089290_成立供應商永續學院.pdf' },
  expectedFilenameBase: 'I_BSID_王小明_089290_成立供應商永續學院',
  createdAt: '2026-11-02T03:00:00.000Z',
  updatedAt: '2026-11-02T03:00:00.000Z',
  version: 1,
  ...over,
});

async function open(me: typeof USER | typeof ADMIN = USER) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline: null, state: 'OPEN', serverTime: '2026-11-08T00:00:00.000Z' })),
  );
  await initI18n('en');
  const router = createMemoryRouter(routes, { initialEntries: [`/ideas/${ID}`] });
  const view = render(<App router={router} client={new QueryClient()} />);
  return { user: userEvent.setup(), ...view };
}

let saved: { blob: Blob; name: string }[];
beforeEach(() => {
  saved = [];
  URL.createObjectURL = vi.fn(() => 'blob:x');
  URL.revokeObjectURL = vi.fn();
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    saved.push({ blob: new Blob(), name: this.download });
  });
});

afterEach(() => vi.restoreAllMocks());

const file = (headers: Record<string, string> = {}, body = 'PDFDATA') =>
  http.get(`${API}/ideas/${ID}/slides`, () => new HttpResponse(body, { headers: { 'Content-Type': 'application/pdf', ...headers } }));

describe('Idea detail', () => {
  it('shows the Idea, members, pending requests and the expected file names', async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())));
    const { container } = await open();
    expect(await screen.findByRole('heading', { level: 1, name: '成立供應商永續學院' })).toBeInTheDocument();
    expect(screen.getByText('Cool Idea Award')).toBeInTheDocument();
    expect(within(screen.getByRole('list', { name: 'Team' })).getByText('李小華')).toBeInTheDocument();
    expect(within(screen.getByRole('list', { name: /Join requests/ })).getByText('陳大文')).toBeInTheDocument();
    expect(screen.getByText('I_BSID_王小明_089290_成立供應商永續學院.pptx')).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect((await axe(container)).violations).toEqual([]);
  });

  it('hides the pending section when pendingRequests is null', async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail({ pendingRequests: null }))));
    await open();
    await screen.findByRole('heading', { name: 'Team' });
    expect(screen.queryByRole('heading', { name: /Join requests/ })).not.toBeInTheDocument();
  });

  it('a 404 shows the one 404 page', async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => problem(404, 'NOT_FOUND')));
    await open();
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
  });

  it('an Admin does not refetch on window focus: one open = one GET', async () => {
    let gets = 0;
    server.use(http.get(`${API}/ideas/${ID}`, () => (gets++, HttpResponse.json(detail()))));
    await open(ADMIN);
    await screen.findByRole('heading', { name: 'Team' });
    window.dispatchEvent(new Event('focus'));
    document.dispatchEvent(new Event('visibilitychange'));
    await new Promise((r) => setTimeout(r, 50));
    expect(gets).toBe(1);
  });

  it('shows an Admin download, replace and delete, but no edit link; passes axe', async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())));
    const { container } = await open(ADMIN);
    await screen.findByRole('heading', { name: 'Team' });
    expect(screen.getByRole('button', { name: 'Download' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /replace/i })).toBeEnabled();
    expect(screen.getByRole('button', { name: /delete/i })).toBeEnabled();
    expect(screen.queryByRole('link', { name: /edit/i })).not.toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('an Admin opening from the list sends one GET /ideas/{id}, even after hovering the row', async () => {
    let gets = 0;
    server.use(
      http.get(`${API}/ideas/${ID}`, () => (gets++, HttpResponse.json(detail()))),
      http.get(`${API}/ideas`, () =>
        HttpResponse.json({ items: [{ id: ID, name: 'N', category: 'COOL_IDEA', org: 'BSID', direction: 'TALENT_DEVELOPMENT', leader: { empNo: '089290', name: '王小明' }, createdAt: '2026-11-02T03:00:00.000Z', myRelation: null }], page: 1, size: 20, total: 1 }),
      ),
    );
    const { user } = await open(ADMIN);
    await screen.findByRole('heading', { name: 'Team' }); // the back link moves into the loaded header
    await user.click(screen.getByRole('link', { name: 'Back to All Ideas' }));
    const link = await screen.findByRole('link', { name: 'N' });
    await user.hover(link);
    expect(gets).toBe(1); // the first open only
    await user.click(link);
    await screen.findByRole('heading', { name: 'Team' });
    expect(gets).toBe(1);
  });
});

describe('Slides download', () => {
  const start = async () => {
    server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())));
    const o = await open();
    await o.user.click(await screen.findByRole('button', { name: 'Download' }));
    return o;
  };

  it('saves under the Content-Disposition filename* name', async () => {
    server.use(file({ 'Content-Disposition': `attachment; filename="x.pdf"; filename*=UTF-8''%E7%B0%A1%E5%A0%B1.pdf`, 'Content-Length': '7' }));
    await start();
    await waitFor(() => expect(saved).toHaveLength(1));
    expect(saved[0]?.name).toBe('簡報.pdf');
  });

  it('falls back to the detail downloadFilename without a header', async () => {
    server.use(file());
    await start();
    await waitFor(() => expect(saved[0]?.name).toBe('I_BSID_王小明_089290_成立供應商永續學院.pdf'));
  });

  it('reports a body that breaks off mid-stream as incomplete and saves nothing', async () => {
    server.use(
      http.get(`${API}/ideas/${ID}/slides`, () => {
        const body = new ReadableStream({
          start(c) {
            c.enqueue(new TextEncoder().encode('PDF'));
            c.error(new Error('aborted'));
          },
        });
        return new HttpResponse(body, { headers: { 'Content-Type': 'application/pdf' } });
      }),
    );
    await start();
    expect(await screen.findByRole('alert')).toHaveTextContent('did not finish');
    expect(saved).toHaveLength(0);
  });

  it('reports a body shorter than Content-Length as incomplete', async () => {
    const real = globalThis.fetch;
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
      if (!String(input).endsWith('/slides')) return real(input, init);
      return { ok: true, status: 200, headers: new Headers({ 'Content-Length': '999' }), blob: async () => new Blob(['PDF']) } as Response;
    });
    server.use(http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(detail())));
    const { user } = await open();
    await user.click(await screen.findByRole('button', { name: 'Download' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('did not finish');
    expect(saved).toHaveLength(0);
  });

  it.each([
    [503, 'STORAGE_UNAVAILABLE', 'File storage is temporarily unavailable'],
    [404, 'NOT_FOUND', 'was not found'],
  ] as const)('shows a normal message for %i %s', async (status, code, text) => {
    server.use(http.get(`${API}/ideas/${ID}/slides`, () => problem(status, code)));
    await start();
    expect(await screen.findByRole('alert')).toHaveTextContent(text);
    expect(saved).toHaveLength(0);
  });
});
