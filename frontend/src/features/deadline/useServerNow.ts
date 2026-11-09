import { useMemo } from 'react';
import { makeServerClock, type ServerClock } from '@/shared/time/offset';
import { useDeadline } from './useDeadline';

/**
 * Server-corrected "now" from the deadline response's `serverTime` (Frontend.md §5.3), for
 * relative times such as "requested 2 hours ago". Null until the deadline has loaded: callers then
 * show the absolute time only. The clock starts on first use; the time between the response and
 * then is added once, so a cached response does not make the clock run late.
 */
export function useServerNow(): (() => number) | null {
  const { data, dataUpdatedAt } = useDeadline();
  const serverTime = data?.serverTime;
  return useMemo(() => {
    if (!serverTime) return null;
    let clock: ServerClock | undefined;
    return () => {
      clock ??= makeServerClock(new Date(Date.parse(serverTime) + Math.max(0, Date.now() - dataUpdatedAt)).toISOString());
      return clock.now();
    };
  }, [serverTime, dataUpdatedAt]);
}
