import { apiFetch, type components } from '@/shared/api';

export type IdeaDetail = components['schemas']['IdeaDetail'];

export const ideaKey = (id: string) => ['idea', id] as const;

export async function fetchIdea(id: string, signal?: AbortSignal): Promise<IdeaDetail> {
  const res = await apiFetch(`/ideas/${encodeURIComponent(id)}`, { signal });
  return (await res.json()) as IdeaDetail;
}

/** The download stopped before the announced size (Frontend.md §5.5): the file must not be offered. */
export class IncompleteDownloadError extends Error {
  constructor() {
    super('incomplete download');
  }
}

/** `filename*=UTF-8''…` (RFC 6266) first, then the plain `filename=`; null when neither parses. */
export function filenameFromDisposition(header: string | null): string | null {
  if (header === null) return null;
  const star = /filename\*\s*=\s*UTF-8''([^;]+)/i.exec(header);
  if (star?.[1]) {
    try {
      return decodeURIComponent(star[1].trim());
    } catch {
      /* fall through to the ASCII fallback */
    }
  }
  const plain = /filename\s*=\s*"((?:[^"\\]|\\.)*)"/i.exec(header) ?? /filename\s*=\s*([^;]+)/i.exec(header);
  return plain?.[1] ? plain[1].replace(/\\(.)/g, '$1').trim() : null;
}

/**
 * fetch → blob with a size check. A body shorter than `Content-Length` (the server aborts after
 * the headers when storage fails mid-stream) throws `IncompleteDownloadError`. Errors before the
 * first byte are ordinary `ApiError`s (404, 503), never a raw JSON page.
 */
export async function fetchSlides(id: string, fallbackName: string, signal?: AbortSignal): Promise<{ blob: Blob; filename: string }> {
  const res = await apiFetch(`/ideas/${encodeURIComponent(id)}/slides`, { headers: { Accept: '*/*' }, signal });
  let blob: Blob;
  try {
    blob = await res.blob();
  } catch (e) {
    if (signal?.aborted) throw e;
    throw new IncompleteDownloadError();
  }
  const announced = Number(res.headers.get('Content-Length'));
  if (Number.isFinite(announced) && res.headers.has('Content-Length') && blob.size < announced) throw new IncompleteDownloadError();
  return { blob, filename: filenameFromDisposition(res.headers.get('Content-Disposition')) ?? fallbackName };
}

/** Hands the blob to the browser through a temporary `<a download>`. */
export function saveBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.hidden = true;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
