import type { HTMLAttributes } from 'react';

import styles from './Mono.module.css';

export type MonoTone = 'accent' | 'soft' | 'muted' | 'dim' | 'paused' | 'danger';

interface MonoProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: MonoTone;
  /** Data (sizes, rates, names) rather than a spaced-out label. */
  data?: boolean;
}

/** JetBrains Mono text: section labels, and anything that is data. */
export function Mono({ tone = 'dim', data = false, className, ...rest }: MonoProps) {
  return (
    <span
      className={[styles.mono, styles[tone], data && styles.data, className].filter(Boolean).join(' ')}
      {...rest}
    />
  );
}
