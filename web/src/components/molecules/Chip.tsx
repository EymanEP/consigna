import type { ButtonHTMLAttributes } from 'react';

import styles from './Chip.module.css';

/** A glowing pill button, e.g. "1 NEW FILE". */
export function Chip({ children, className, ...rest }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button type="button" className={[styles.chip, className].filter(Boolean).join(' ')} {...rest}>
      <span className={styles.dot} aria-hidden="true" />
      {children}
    </button>
  );
}
