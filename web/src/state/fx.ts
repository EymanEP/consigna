import { createStore } from '../lib/store';
import { safeStorage } from '../lib/storage';

/** Visual effects level: full CRT motion, or a calm, static screen. */
export type FxLevel = 'full' | 'calm';

const KEY = 'consigna.fx';

function initial(): FxLevel {
  const saved = safeStorage.get(KEY);
  if (saved === 'full' || saved === 'calm') return saved;
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'calm' : 'full';
}

export const fx = createStore<FxLevel>(typeof window === 'undefined' ? 'calm' : initial());

export function setFx(level: FxLevel): void {
  fx.set(level);
  safeStorage.set(KEY, level);
}
