import { describe, expect, it, vi } from 'vitest';

import { createStore } from './store';

describe('createStore', () => {
  it('notifies subscribers on change only', () => {
    const s = createStore({ n: 1 });
    const listener = vi.fn();
    const unsubscribe = s.subscribe(listener);
    s.set((prev) => ({ n: prev.n + 1 }));
    expect(s.get()).toEqual({ n: 2 });
    expect(listener).toHaveBeenCalledTimes(1);
    const same = s.get();
    s.set(same);
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
    s.set({ n: 3 });
    expect(listener).toHaveBeenCalledTimes(1);
  });
});
