import styles from './StatusDot.module.css';

interface StatusDotProps {
  state?: 'live' | 'off' | 'paused';
  pulse?: boolean;
}

/** A small phosphor dot. Decorative: pair it with text that says the state. */
export function StatusDot({ state = 'live', pulse = false }: StatusDotProps) {
  const cls = [
    styles.dot,
    state === 'off' && styles.off,
    state === 'paused' && styles.paused,
    pulse && styles.pulse,
  ]
    .filter(Boolean)
    .join(' ');
  return <span className={cls} aria-hidden="true" />;
}
