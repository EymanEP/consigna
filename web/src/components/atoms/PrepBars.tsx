import styles from './PrepBars.module.css';

/** Five bouncing bars: the "working on it" indicator. Decorative. */
export function PrepBars({ small = false }: { small?: boolean }) {
  return (
    <span className={[styles.bars, small && styles.small].filter(Boolean).join(' ')} aria-hidden="true">
      {[0, 1, 2, 3, 4].map((i) => (
        <span key={i} className={styles.bar} />
      ))}
    </span>
  );
}
