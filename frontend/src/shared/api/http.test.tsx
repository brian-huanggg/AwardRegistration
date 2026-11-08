import { http, HttpResponse } from 'msw';
import { problem } from './mocks/problem';
import { server } from './mocks/server';
import { ApiError } from './problem';
import { clearCsrfToken } from './csrf';
import { apiFetch, setUnauthenticatedHandler } from './http';

const API = 'http://localhost:3000/api/v1';

/** Counts GET /auth/csrf and hands out t1, t2, … */
function csrfEndpoint() {
  const state = { calls: 0 };
  server.use(
    http.get(`${API}/auth/csrf`, () => {
      state.calls += 1;
      return HttpResponse.json({ token: `t${state.calls}` });
    }),
  );
  return state;
}

beforeEach(() => {
  clearCsrfToken();
  setUnauthenticatedHandler(null);
});

describe('CSRF token', () => {
  it('is fetched lazily, cached, and sent on writes only', async () => {
    const csrf = csrfEndpoint();
    const seen: (string | null)[] = [];
    server.use(
      http.get(`${API}/deadline`, ({ request }) => {
        seen.push(request.headers.get('X-CSRF-Token'));
        return HttpResponse.json({});
      }),
      http.post(`${API}/join-requests`, ({ request }) => {
        seen.push(request.headers.get('X-CSRF-Token'));
        return HttpResponse.json({}, { status: 201 });
      }),
      http.delete(`${API}/ideas/x`, ({ request }) => {
        seen.push(request.headers.get('X-CSRF-Token'));
        return new HttpResponse(null, { status: 204 });
      }),
    );
    await apiFetch('/deadline');
    expect(csrf.calls).toBe(0);
    await apiFetch('/join-requests', { method: 'POST', body: { a: 1 } });
    await apiFetch('/ideas/x', { method: 'DELETE' });
    expect(csrf.calls).toBe(1);
    expect(seen).toEqual([null, 't1', 't1']);
  });

  it('is shared by concurrent first writes', async () => {
    const csrf = csrfEndpoint();
    server.use(http.post(`${API}/join-requests`, () => HttpResponse.json({}, { status: 201 })));
    await Promise.all([1, 2, 3].map(() => apiFetch('/join-requests', { method: 'POST', body: {} })));
    expect(csrf.calls).toBe(1);
  });

  it('sends JSON bodies with a JSON content type', async () => {
    csrfEndpoint();
    let ct: string | null = null;
    let body: unknown;
    server.use(
      http.post(`${API}/auth/register`, async ({ request }) => {
        ct = request.headers.get('Content-Type');
        body = await request.json();
        return HttpResponse.json({}, { status: 201 });
      }),
    );
    await apiFetch('/auth/register', { method: 'POST', body: { empNo: '1' } });
    expect(ct).toBe('application/json');
    expect(body).toEqual({ empNo: '1' });
  });

  it.each(['/auth/login', '/auth/logout'])('is refetched after a successful %s', async (path) => {
    const csrf = csrfEndpoint();
    const seen: string[] = [];
    server.use(
      http.post(`${API}${path}`, ({ request }) => {
        seen.push(request.headers.get('X-CSRF-Token') ?? '');
        return path === '/auth/login' ? HttpResponse.json({}) : new HttpResponse(null, { status: 204 });
      }),
    );
    await apiFetch(path, { method: 'POST', body: {} });
    await apiFetch(path, { method: 'POST', body: {} });
    expect(csrf.calls).toBe(2);
    expect(seen).toEqual(['t1', 't2']);
  });

  it('is kept after a failed login', async () => {
    const csrf = csrfEndpoint();
    server.use(http.post(`${API}/auth/login`, () => problem(401, 'INVALID_CREDENTIALS')));
    const unauth = vi.fn();
    setUnauthenticatedHandler(unauth);
    await expect(apiFetch('/auth/login', { method: 'POST', body: {} })).rejects.toMatchObject({ code: 'INVALID_CREDENTIALS' });
    await expect(apiFetch('/auth/login', { method: 'POST', body: {} })).rejects.toBeInstanceOf(ApiError);
    expect(csrf.calls).toBe(1);
    expect(unauth).not.toHaveBeenCalled();
  });
});

describe('CSRF_INVALID', () => {
  it('refetches and retries once, resending the same body', async () => {
    const csrf = csrfEndpoint();
    const attempts: { token: string | null; body: unknown }[] = [];
    server.use(
      http.post(`${API}/join-requests`, async ({ request }) => {
        attempts.push({ token: request.headers.get('X-CSRF-Token'), body: await request.json() });
        return attempts.length === 1 ? problem(403, 'CSRF_INVALID') : HttpResponse.json({}, { status: 201 });
      }),
    );
    const res = await apiFetch('/join-requests', { method: 'POST', body: { ideaId: 'i' } });
    expect(res.status).toBe(201);
    expect(csrf.calls).toBe(2);
    expect(attempts).toEqual([
      { token: 't1', body: { ideaId: 'i' } },
      { token: 't2', body: { ideaId: 'i' } },
    ]);
  });

  it('does not retry a second time', async () => {
    const csrf = csrfEndpoint();
    let posts = 0;
    server.use(
      http.post(`${API}/join-requests`, () => {
        posts += 1;
        return problem(403, 'CSRF_INVALID');
      }),
    );
    await expect(apiFetch('/join-requests', { method: 'POST', body: {} })).rejects.toMatchObject({ status: 403, code: 'CSRF_INVALID' });
    expect(posts).toBe(2);
    expect(csrf.calls).toBe(2);
  });

  it('does not retry other 403s', async () => {
    const csrf = csrfEndpoint();
    let posts = 0;
    server.use(
      http.post(`${API}/join-requests`, () => {
        posts += 1;
        return problem(403, 'SUBMISSION_CLOSED');
      }),
    );
    await expect(apiFetch('/join-requests', { method: 'POST', body: {} })).rejects.toMatchObject({ code: 'SUBMISSION_CLOSED' });
    expect(posts).toBe(1);
    expect(csrf.calls).toBe(1);
  });

  it('resends a multipart body on retry', async () => {
    csrfEndpoint();
    const names: (string | null)[] = [];
    server.use(
      http.post(`${API}/ideas`, async ({ request }) => {
        const f = (await request.formData()).get('title');
        names.push(f as string);
        return names.length === 1 ? problem(403, 'CSRF_INVALID') : HttpResponse.json({}, { status: 201 });
      }),
    );
    const fd = new FormData();
    fd.set('title', 'T');
    await apiFetch('/ideas', { method: 'POST', body: fd });
    expect(names).toEqual(['T', 'T']);
  });

  it('a failed token fetch surfaces its error and caches nothing', async () => {
    server.use(http.get(`${API}/auth/csrf`, () => problem(429, 'RATE_LIMITED')));
    await expect(apiFetch('/join-requests', { method: 'POST', body: {} })).rejects.toMatchObject({ status: 429 });
    const csrf = csrfEndpoint();
    server.use(http.post(`${API}/join-requests`, () => HttpResponse.json({}, { status: 201 })));
    await apiFetch('/join-requests', { method: 'POST', body: {} });
    expect(csrf.calls).toBe(1);
  });
});

describe('401', () => {
  it('clears the token and calls the handler', async () => {
    const csrf = csrfEndpoint();
    const unauth = vi.fn();
    setUnauthenticatedHandler(unauth);
    let n = 0;
    server.use(
      http.post(`${API}/join-requests`, () => {
        n += 1;
        return n === 1 ? problem(401, 'UNAUTHENTICATED') : HttpResponse.json({}, { status: 201 });
      }),
    );
    await expect(apiFetch('/join-requests', { method: 'POST', body: {} })).rejects.toMatchObject({ status: 401, code: 'UNAUTHENTICATED' });
    expect(unauth).toHaveBeenCalledTimes(1);
    await apiFetch('/join-requests', { method: 'POST', body: {} });
    expect(csrf.calls).toBe(2);
  });

  it('is an answer, not a redirect, when allowUnauthenticated is set', async () => {
    const unauth = vi.fn();
    setUnauthenticatedHandler(unauth);
    server.use(http.get(`${API}/auth/me`, () => problem(401, 'UNAUTHENTICATED')));
    await expect(apiFetch('/auth/me', { allowUnauthenticated: true })).rejects.toMatchObject({ status: 401 });
    expect(unauth).not.toHaveBeenCalled();
  });

  it('fires for reads too', async () => {
    const unauth = vi.fn();
    setUnauthenticatedHandler(unauth);
    server.use(http.get(`${API}/ideas`, () => problem(401, 'UNAUTHENTICATED')));
    await expect(apiFetch('/ideas')).rejects.toBeInstanceOf(ApiError);
    expect(unauth).toHaveBeenCalledTimes(1);
  });
});
