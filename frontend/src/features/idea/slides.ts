import { apiFetch, xhrTransport } from '@/shared/api';
import type { IdeaDetail } from './detail';

/** PUT /ideas/{id}/slides: one multipart part `slides`; the response is the new detail (PRD §6.6, §6.8). */
export async function replaceSlides(id: string, file: File, onProgress?: (loaded: number, total: number) => void): Promise<IdeaDetail> {
  const body = new FormData();
  body.append('slides', file, file.name);
  const res = await apiFetch(`/ideas/${encodeURIComponent(id)}/slides`, { method: 'PUT', body, transport: xhrTransport(onProgress) });
  return (await res.json()) as IdeaDetail;
}

/** DELETE /ideas/{id}: 204 with no body. */
export async function deleteIdea(id: string): Promise<void> {
  await apiFetch(`/ideas/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
