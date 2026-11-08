import { HttpResponse } from 'msw';

/** Test helper: an application/problem+json response as the backend sends it. */
export function problem(status: number, code: string, extra: Record<string, unknown> = {}, headers: Record<string, string> = {}) {
  return HttpResponse.json(
    { type: 'about:blank', title: 'Error', status, code, detail: 'x', requestId: '6f1c2c1e-0000-4000-8000-000000000000', ...extra },
    { status, headers: { 'Content-Type': 'application/problem+json', ...headers } },
  );
}
