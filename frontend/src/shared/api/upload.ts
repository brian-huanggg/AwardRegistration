/**
 * Upload transport on raw XMLHttpRequest (Frontend.md F6): `fetch` has no upload
 * progress, and PRD §8.5 asks for a progress bar. It is a drop-in for the `fetch`
 * that `apiFetch` uses, so CSRF, the `CSRF_INVALID` retry, 401 handling and error
 * parsing are the same as for every other write.
 */
export type Transport = (url: string, init: RequestInit) => Promise<Response>;

/** A transport that reports `loaded / total` bytes of the request body while it is sent. */
export function xhrTransport(onProgress?: (loaded: number, total: number) => void): Transport {
  return (url, init) =>
    new Promise<Response>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open(init.method ?? 'POST', url);
      new Headers(init.headers).forEach((value, name) => xhr.setRequestHeader(name, value));
      xhr.withCredentials = true;
      xhr.responseType = 'text';
      if (onProgress) {
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) onProgress(e.loaded, e.total);
        };
      }
      xhr.onload = () => {
        const headers = new Headers();
        for (const line of xhr.getAllResponseHeaders().trim().split(/\r?\n/)) {
          const at = line.indexOf(':');
          if (at > 0) headers.append(line.slice(0, at).trim(), line.slice(at + 1).trim());
        }
        const empty = xhr.status === 204 || xhr.status === 205 || xhr.status === 304;
        resolve(new Response(empty ? null : xhr.responseText, { status: xhr.status, headers }));
      };
      // The request may or may not have reached the server: the caller treats this as an unknown outcome.
      xhr.onerror = () => reject(new TypeError('Network error'));
      xhr.ontimeout = () => reject(new TypeError('Network timeout'));
      xhr.onabort = () => reject(new DOMException('Aborted', 'AbortError'));
      init.signal?.addEventListener('abort', () => xhr.abort(), { once: true });
      if (init.signal?.aborted) xhr.abort();
      else xhr.send(init.body as XMLHttpRequestBodyInit | null);
    });
}
