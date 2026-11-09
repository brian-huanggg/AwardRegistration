import { useSyncExternalStore } from 'react';

/** ≥ 640 px (Tailwind `sm`): table layout; below it the list renders cards (UI-03). */
export const WIDE_QUERY = '(min-width: 640px)';

const media = () => (typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia(WIDE_QUERY) : null);

function subscribe(onChange: () => void) {
  const m = media();
  m?.addEventListener('change', onChange);
  return () => m?.removeEventListener('change', onChange);
}

/**
 * True when the viewport is at least 640 px wide. Without `matchMedia` (jsdom) it reports wide, so
 * only one layout is ever in the DOM and assistive tech never meets the same links twice.
 */
export function useWide(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => media()?.matches ?? true,
    () => true,
  );
}
