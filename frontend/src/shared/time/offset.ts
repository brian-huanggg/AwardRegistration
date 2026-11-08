/** Server-clock offset (Frontend.md §5.3): offset = serverTime − clientNow, advanced by performance.now(). */
export interface ServerClock {
  /** Current server time in epoch ms, immune to later changes of the system clock. */
  now(): number;
}

export function makeServerClock(serverTimeIso: string): ServerClock {
  const base = Date.parse(serverTimeIso);
  const started = performance.now();
  return { now: () => base + (performance.now() - started) };
}
