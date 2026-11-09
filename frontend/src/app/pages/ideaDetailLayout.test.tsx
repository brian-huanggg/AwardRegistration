import { QueryClient } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter } from 'react-router';
import { axe } from 'vitest-axe';
import { clearCsrfToken } from '@/shared/api';
import { server } from '@/shared/api/mocks/server';
import { initI18n } from '@/shared/i18n';
import { App } from '../App';
import { routes } from '../router';

// UI-04 (Pencil rsQOw): header actions, cards and alerts per role. One test per permission-matrix
// cell the redesign touches (PRD §7, can.ts): edit, delete, leave, replace, approve, reject, remove.

const API = 'http://localhost:3000/api/v1';
const ID = '0193a1b2-0000-7000-8000-000000000001';
const LEADER = { empNo: '089290', name: '王小明', email: 'wang@example.com', role: 'USER' };
const MEMBER = { empNo: '089291', name: '李小華', email: 'lee@example.com', role: 'USER' };
const ADMIN = { empNo: '000001', name: 'Admin', email: null, role: 'ADMIN' };
type Me = typeof LEADER | typeof ADMIN;

const member = (empNo: string, name: string) => ({ empNo, name, joinedAt: '2026-11-03T03:00:00.000Z' });

const detail = (over: Record<string, unknown> = {}) => ({
  id: ID,
  name: '成立供應商永續學院',
  category: 'COOL_IDEA',
  org: 'BSID',
  direction: 'TALENT_DEVELOPMENT',
  summary: 's',
  leader: { empNo: '089290', name: '王小明' },
  members: [member('089291', '李小華')],
  pendingRequests: [{ empNo: '089292', name: '陳大文', requestedAt: '2026-11-04T03:00:00.000Z' }],
  slides: { ext: 'pdf', size: 2048, sha256: 'x', updatedAt: '2026-11-02T03:00:00.000Z', downloadFilename: 'I_BSID_王小明_089290_成立供應商永續學院.pdf' },
  expectedFilenameBase: 'I_BSID_王小明_089290_成立供應商永續學院',
  createdAt: '2026-11-02T03:00:00.000Z',
  updatedAt: '2026-11-02T03:00:00.000Z',
  version: 1,
  ...over,
});

async function open(
  me: Me,
  { idea = detail(), state = 'OPEN', deadline = '2026-11-30T10:00:00.000Z', entry = { pathname: `/ideas/${ID}` } as { pathname: string; state?: unknown } } = {},
) {
  clearCsrfToken();
  server.use(
    http.get(`${API}/auth/me`, () => HttpResponse.json(me)),
    http.get(`${API}/deadline`, () => HttpResponse.json({ deadline, state, serverTime: '2026-11-04T05:00:00.000Z' })),
    http.get(`${API}/ideas/${ID}`, () => HttpResponse.json(idea)),
  );
  await initI18n('en');
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  const view = render(<App router={router} client={new QueryClient()} />);
  await screen.findByRole('heading', { name: 'Team' });
  return { main: within(screen.getByRole('main')), ...view };
}

const team = () => within(screen.getByRole('list', { name: 'Team' }));

describe('Idea detail layout (UI-04)', () => {
  it('Leader: edit and delete in the header, no leave; team and join requests cards; passes axe', async () => {
    const { main, container } = await open(LEADER);
    expect(screen.getByRole('heading', { level: 1, name: '成立供應商永續學院' })).toBeInTheDocument();
    expect(main.getByText('You are the Leader')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Edit details' })).toHaveAttribute('href', `/ideas/${ID}/edit`);
    expect(screen.getByRole('button', { name: 'Delete Idea' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Replace slides' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: 'Leave team' })).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /Join requests/ })).toBeInTheDocument();
    expect(screen.getByText('Visible only to you and admins')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove 李小華 (089291)' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Approve 陳大文 (089292)' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Reject 陳大文 (089292)' })).toBeEnabled();
    expect(team().getByText('(you)').closest('li')).toHaveTextContent('王小明');
    expect(screen.getByRole('link', { name: 'Back to My Ideas' })).toHaveAttribute('href', '/ideas');
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect((await axe(container)).violations).toEqual([]);
  });

  it('Member: leave in the header, no edit/delete/replace, no member controls, no join requests card; passes axe', async () => {
    const { main, container } = await open(MEMBER, { idea: detail({ pendingRequests: null }) });
    expect(main.getByText('You are a Member')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Leave team' })).toBeEnabled();
    expect(screen.queryByRole('link', { name: 'Edit details' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Delete Idea|Replace slides|Remove|Approve|Reject/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /Join requests/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/open seat/)).not.toBeInTheDocument();
    expect(team().getByText('(you)').closest('li')).toHaveTextContent('李小華');
    expect(screen.getByRole('button', { name: 'Download' })).toBeEnabled();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('Admin: delete and replace, no edit/leave/team controls; read-only join requests; passes axe', async () => {
    const { main, container } = await open(ADMIN);
    expect(main.getByText('Administrator')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete Idea' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Replace slides' })).toBeEnabled();
    expect(screen.queryByRole('link', { name: 'Edit details' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Leave|Remove|Approve|Reject/ })).not.toBeInTheDocument();
    expect(screen.getByText('Visible only to the Leader and admins')).toBeInTheDocument();
    expect(screen.queryByText(/Approving checks capacity/)).not.toBeInTheDocument();
    expect(screen.queryByText(/open seat/)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Back to All Ideas' })).toBeInTheDocument();
    expect((await axe(container)).violations).toEqual([]);
  });

  it('Leader controls are shown but disabled when CLOSED; the alert says changes are locked', async () => {
    await open(LEADER, { state: 'CLOSED' });
    for (const name of ['Delete Idea', 'Replace slides', 'Remove 李小華 (089291)', 'Approve 陳大文 (089292)', 'Reject 陳大文 (089292)'])
      expect(screen.getByRole('button', { name })).toBeDisabled();
    const note = screen.getByRole('note');
    expect(note).toHaveTextContent('Changes are locked');
    expect(note).not.toHaveTextContent('Changes lock on');
  });

  it('Member leave is disabled when CLOSED', async () => {
    await open(MEMBER, { state: 'CLOSED', idea: detail({ pendingRequests: null }) });
    expect(screen.getByRole('button', { name: 'Leave team' })).toBeDisabled();
  });

  it('OPEN shows "Changes lock on" with the deadline; NOT_OPEN shows no alert', async () => {
    const a = await open(LEADER);
    const note = screen.getByRole('note');
    expect(note).toHaveTextContent(/Changes lock on .*2026/);
    expect(within(note).getByText(/Changes lock on/).closest('time')).toHaveAttribute('dateTime', '2026-11-30T10:00:00.000Z');
    a.unmount();
    await open(LEADER, { state: 'NOT_OPEN', deadline: null as unknown as string });
    expect(screen.queryByRole('note')).not.toBeInTheDocument();
  });

  it('the join requests card shows the empty text for []', async () => {
    await open(LEADER, { idea: detail({ pendingRequests: [] }) });
    expect(screen.getByRole('heading', { name: /Join requests/ })).toHaveTextContent('0');
    expect(screen.getByText('No pending requests.')).toBeInTheDocument();
  });

  it('seat count and open-seat hint for the Leader', async () => {
    await open(LEADER);
    expect(screen.getByText('2 of 4 seats filled')).toBeInTheDocument();
    expect(screen.getByText(/2 open seats — teammates apply with your employee ID 089290/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Copy the Idea name' })).toBeInTheDocument();
  });

  it('at 4/4 the approve button and the open-seat hint are hidden; reject stays', async () => {
    await open(LEADER, { idea: detail({ members: [member('089291', '李小華'), member('089293', '林一'), member('089294', '張二')] }) });
    expect(screen.getByText('4 of 4 seats filled')).toBeInTheDocument();
    expect(screen.queryByText(/open seat/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Approve/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reject 陳大文 (089292)' })).toBeEnabled();
  });

  it('request time is relative to the server clock, with the absolute time kept', async () => {
    await open(LEADER);
    const time = within(screen.getByRole('list', { name: /Join requests/ })).getByText('requested 2 hours ago');
    expect(time.tagName).toBe('TIME');
    expect(time).toHaveAttribute('dateTime', '2026-11-04T03:00:00.000Z');
    expect(time).toHaveAttribute('title');
  });

  it('the back link returns to the list URL from history state', async () => {
    await open(LEADER, { entry: { pathname: `/ideas/${ID}`, state: { from: '/ideas?page=2' } } });
    expect(screen.getByRole('link', { name: 'Back to My Ideas' })).toHaveAttribute('href', '/ideas?page=2');
  });
});
