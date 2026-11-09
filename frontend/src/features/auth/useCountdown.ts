import { useCallback, useEffect, useState } from "react";

/**
 * Whole seconds left of a countdown started with `start(seconds)`. It advances with
 * `performance.now()`, so changing the system clock does not affect it (Frontend.md §5.3).
 */
export function useCountdown() {
  const [endsAt, setEndsAt] = useState<number | null>(null);
  const [remaining, setRemaining] = useState(0);

  useEffect(() => {
    if (endsAt === null) return;
    const tick = () => {
      const left = Math.max(0, Math.ceil((endsAt - performance.now()) / 1000));
      setRemaining(left);
      if (left === 0) setEndsAt(null);
    };
    tick();
    const id = setInterval(tick, 250);
    return () => clearInterval(id);
  }, [endsAt]);

  const start = useCallback((seconds: number) => {
    setEndsAt(performance.now() + Math.max(1, seconds) * 1000);
  }, []);

  return { remaining, start };
}
