/**
 * Time of the last answered server request (Frontend.md §8.1, OI-22). Any answer except a 401 means
 * the server saw the session, so the idle clock restarts; the idle-timeout warning reads it.
 */
let last = Date.now();
const listeners = new Set<() => void>();

export const lastActivity = () => last;

export function markActivity(): void {
  last = Date.now();
  listeners.forEach((fn) => fn());
}

export function subscribeActivity(fn: () => void): () => void {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}
