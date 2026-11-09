import '@testing-library/jest-dom/vitest';
import { server } from '@/shared/api/mocks/server';

/**
 * jsdom replaces FormData/File with its own classes, which Node's fetch cannot
 * serialise (the multipart body is never written, so MSW fails with "Premature
 * close"). Real browsers send a FormData natively; here the jsdom one is encoded
 * to multipart/form-data (RFC 7578) just before the request reaches MSW. It is
 * installed after `server.listen()` so that it sits on top of MSW's own fetch
 * interceptor.
 */
async function encodeMultipart(body: FormData): Promise<{ bytes: Uint8Array<ArrayBuffer>; contentType: string }> {
  const boundary = `----vitest${Math.random().toString(16).slice(2)}`;
  const enc = new TextEncoder();
  const quote = (s: string) => s.replace(/"/g, '%22').replace(/\r/g, '%0D').replace(/\n/g, '%0A');
  const parts: Uint8Array[] = [];
  for (const [key, value] of body.entries()) {
    const disposition = `Content-Disposition: form-data; name="${quote(key)}"`;
    if (typeof value === 'string') {
      parts.push(enc.encode(`--${boundary}\r\n${disposition}\r\n\r\n`), enc.encode(value), enc.encode('\r\n'));
    } else {
      const head = `--${boundary}\r\n${disposition}; filename="${quote(value.name)}"\r\nContent-Type: ${value.type || 'application/octet-stream'}\r\n\r\n`;
      parts.push(enc.encode(head), new Uint8Array(await value.arrayBuffer()), enc.encode('\r\n'));
    }
  }
  parts.push(enc.encode(`--${boundary}--\r\n`));
  const bytes = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let at = 0;
  for (const p of parts) {
    bytes.set(p, at);
    at += p.length;
  }
  return { bytes, contentType: `multipart/form-data; boundary=${boundary}` };
}

let mswFetch: typeof fetch;

beforeAll(() => {
  server.listen({ onUnhandledFrame: 'error' });
  mswFetch = globalThis.fetch;
  globalThis.fetch = async (input, init) => {
    if (!(init?.body instanceof FormData)) return mswFetch(input, init);
    const { bytes, contentType } = await encodeMultipart(init.body);
    const headers = new Headers(init.headers);
    headers.set('Content-Type', contentType);
    return mswFetch(input, { ...init, headers, body: bytes });
  };
});
afterEach(() => server.resetHandlers());
afterAll(() => {
  globalThis.fetch = mswFetch;
  server.close();
});
