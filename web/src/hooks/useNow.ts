import { useEffect, useState } from 'react';

/** The current time, refreshed every interval ms, shifted by offset ms. */
export function useNow(interval = 30_000, offset = 0): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), interval);
    return () => clearInterval(t);
  }, [interval]);
  return now + offset;
}
