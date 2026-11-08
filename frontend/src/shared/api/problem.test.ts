import { http, HttpResponse } from 'msw';
import codes from 'testdata/error-codes.json';
import { ERROR_CODES } from './error-codes.generated';
import { problem } from './mocks/problem';
import { server } from './mocks/server';
import { parseApiError } from './problem';

const get = (path = '/x') => fetch(`http://localhost${path}`);

describe('generated error codes', () => {
  it('match testdata/error-codes.json', () => {
    expect(ERROR_CODES).toEqual(codes);
  });
});

describe('parseApiError: one case per problem type', () => {
  it.each(codes)('parses problem+json $code ($status)', async ({ code, status }) => {
    server.use(http.get('http://localhost/x', () => problem(status, code)));
    const err = await parseApiError(await get());
    expect(err.code).toBe(code);
    expect(err.status).toBe(status);
    expect(err.requestId).toBe('6f1c2c1e-0000-4000-8000-000000000000');
  });

  it('keeps errors[] in order with extra members', async () => {
    const errors = [
      { field: 'slides', code: 'FILENAME_MISMATCH', expected: 'a.pdf' },
      { field: 'name', code: 'REQUIRED' },
    ];
    server.use(http.get('http://localhost/x', () => problem(422, 'FILENAME_MISMATCH', { errors })));
    const err = await parseApiError(await get());
    expect(err.errors).toEqual(errors);
  });

  it('drops malformed errors[] entries', async () => {
    server.use(http.get('http://localhost/x', () => problem(422, 'VALIDATION_FAILED', { errors: [1, { field: 'a' }, { field: 'a', code: 'REQUIRED' }] })));
    expect((await parseApiError(await get())).errors).toEqual([{ field: 'a', code: 'REQUIRED' }]);
  });

  it('reads Retry-After on 429', async () => {
    server.use(http.get('http://localhost/x', () => problem(429, 'LOGIN_THROTTLED', {}, { 'Retry-After': '42' })));
    expect((await parseApiError(await get())).retryAfterSeconds).toBe(42);
  });

  it('ignores a non-numeric Retry-After', async () => {
    server.use(http.get('http://localhost/x', () => problem(429, 'RATE_LIMITED', {}, { 'Retry-After': 'Wed, 21 Oct 2026 07:28:00 GMT' })));
    expect((await parseApiError(await get())).retryAfterSeconds).toBeUndefined();
  });

  it('yields UNKNOWN for an unregistered code', async () => {
    server.use(http.get('http://localhost/x', () => problem(418, 'TEAPOT')));
    expect((await parseApiError(await get())).code).toBe('UNKNOWN');
  });
});

describe('parseApiError: non-problem bodies', () => {
  it('maps a non-JSON 413 from the ingress to FILE_TOO_LARGE', async () => {
    server.use(http.get('http://localhost/x', () => new HttpResponse('<html>too big</html>', { status: 413, headers: { 'Content-Type': 'text/html' } })));
    const err = await parseApiError(await get());
    expect(err.code).toBe('FILE_TOO_LARGE');
    expect(err.status).toBe(413);
  });

  it('maps a plain 503 to SERVICE_UNAVAILABLE', async () => {
    server.use(http.get('http://localhost/x', () => new HttpResponse('', { status: 503 })));
    expect((await parseApiError(await get())).code).toBe('SERVICE_UNAVAILABLE');
  });

  it('falls back to the status when problem+json is broken', async () => {
    server.use(http.get('http://localhost/x', () => new HttpResponse('{oops', { status: 400, headers: { 'Content-Type': 'application/problem+json' } })));
    expect((await parseApiError(await get())).code).toBe('MALFORMED_REQUEST');
  });

  it('uses the X-Request-Id header when the body has none', async () => {
    server.use(http.get('http://localhost/x', () => new HttpResponse('', { status: 500, headers: { 'X-Request-Id': 'abc' } })));
    const err = await parseApiError(await get());
    expect(err.code).toBe('INTERNAL_ERROR');
    expect(err.requestId).toBe('abc');
  });

  it('returns UNKNOWN for an unmapped status', async () => {
    server.use(http.get('http://localhost/x', () => new HttpResponse('', { status: 418 })));
    expect((await parseApiError(await get())).code).toBe('UNKNOWN');
  });
});
