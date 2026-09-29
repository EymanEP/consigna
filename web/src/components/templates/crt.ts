import { createContext, useContext } from 'react';

export interface Crt {
  /** A heavier, one-off glitch that marks a state change (pause, resume). */
  glitch: (strength?: number) => void;
}

export const CrtContext = createContext<Crt>({ glitch: () => undefined });

export function useCrt(): Crt {
  return useContext(CrtContext);
}
