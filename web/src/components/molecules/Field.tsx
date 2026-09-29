import type { ReactNode } from 'react';

import styles from './Field.module.css';

interface FieldProps {
  /** id of the main control, which the label points at. */
  htmlFor: string;
  label: string;
  hint?: string;
  hintId?: string;
  error?: string | null;
  errorId?: string;
  children: ReactNode;
}

/** A labelled form control with optional hint and error text. */
export function Field({ htmlFor, label, hint, hintId, error, errorId, children }: FieldProps) {
  return (
    <div className={styles.field}>
      <label htmlFor={htmlFor} className={styles.label}>
        {label}
      </label>
      <div className={styles.control}>{children}</div>
      {hint && (
        <p id={hintId} className={styles.hint}>
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className={styles.error} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
