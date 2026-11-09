import { markActivity } from './activity';
import { API_BASE, clearCsrfToken, getCsrfToken, refreshCsrfToken } from './csrf';
import { parseApiError, type ApiError } from './problem';
import type { Transport } from './upload';

const SAFE = new Set(['GET', 'HEAD', 'OPTIONS']);

/** Endpoints after which the session binding of the CSRF token changes (PRD §6.5). */
const SESSION_CHANGING = new Set(['/auth/login', '/auth/logout']);

type Unauthenticated = () => void;
let onUnauthenticated: Unauthenticated | null = null;

/** The app registers what a 401 does (clear `me`, go to /login?next=). Returns an unregister function. */
export function setUnauthenticatedHandler(fn: Unauthenticated | null): () => void {
  onUnauthenticated = fn;
  return () => {
    if (onUnauthenticated === fn) onUnauthenticated = null;
  };
}

export interface ApiRequestInit extends Omit<RequestInit, 'body'> {
  /** Plain objects are sent as JSON; strings, FormData and Blobs are sent as given. */
  body?: BodyInit | Record<string, unknown> | null;
  /** A 401 is an answer here (e.g. `GET /auth/me` means "visitor"), not a session end. */
  allowUnauthenticated?: boolean;
  /** Replaces `fetch`, e.g. the XHR transport that reports upload progress. */
  transport?: Transport;
}

function prepare(init: ApiRequestInit): { headers: Headers; body: BodyInit | null | undefined } {
  const headers = new Headers(init.headers);
  if (!headers.has('Accept')) headers.set('Accept', 'application/json');
  let body = init.body as BodyInit | null | undefined;
  if (body !== null && body !== undefined && typeof body === 'object' && !isNativeBody(body)) {
    body = JSON.stringify(body);
    if (!headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  }
  return { headers, body };
}

function isNativeBody(b: object): boolean {
  return (
    b instanceof FormData ||
    b instanceof Blob ||
    b instanceof URLSearchParams ||
    b instanceof ArrayBuffer ||
    ArrayBuffer.isView(b) ||
    b instanceof ReadableStream
  );
}

/**
 * fetch for `/api/v1`: `path` is relative to it (`/auth/me`). Resolves with the
 * Response on 2xx and throws ApiError otherwise.
 *
 * - Writes send `X-CSRF-Token` (lazily fetched, cached in memory).
 * - `CSRF_INVALID` refetches the token and retries the write once. The request
 *   was rejected before processing, so the retry is safe (Frontend.md §5.2).
 *   A request whose body is a stream cannot be replayed and is not retried.
 * - A successful login or logout clears the token (the binding changed).
 * - A 401 clears the token and calls the registered handler, except for
 *   `INVALID_CREDENTIALS` (a wrong password is not a session end).
 */
export async function apiFetch(path: string, init: ApiRequestInit = {}): Promise<Response> {
  const method = (init.method ?? 'GET').toUpperCase();
  const write = !SAFE.has(method);
  const { headers, body } = prepare(init);
  const { transport, allowUnauthenticated, ...rest } = init;
  const send = (csrf: string | null) => {
    const h = new Headers(headers);
    if (csrf !== null) h.set('X-CSRF-Token', csrf);
    return (transport ?? fetch)(`${API_BASE()}${path}`, { ...rest, method, headers: h, body, credentials: 'same-origin' });
  };

  let res = await send(write ? await getCsrfToken() : null);
  if (write && !res.ok && !(body instanceof ReadableStream) && (await errorCode(res)) === 'CSRF_INVALID') {
    res = await send(await refreshCsrfToken());
  }

  if (res.status !== 401) markActivity();
  if (res.ok) {
    if (write && SESSION_CHANGING.has(path)) clearCsrfToken();
    return res;
  }
  const err = await parseApiError(res);
  if (err.status === 401 && err.code !== 'INVALID_CREDENTIALS' && !allowUnauthenticated) {
    clearCsrfToken();
    onUnauthenticated?.();
  }
  throw err;
}

/** Reads the error code without consuming the response. */
async function errorCode(res: Response): Promise<ApiError['code']> {
  return (await parseApiError(res.clone())).code;
}
