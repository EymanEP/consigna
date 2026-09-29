import type { InputHTMLAttributes } from 'react';

import styles from './Checkbox.module.css';

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: string;
}

/** A native checkbox with a 44px hit area and an accessible label. */
export function Checkbox({ label, className, ...rest }: CheckboxProps) {
  return (
    <input
      type="checkbox"
      aria-label={label}
      className={[styles.box, className].filter(Boolean).join(' ')}
      {...rest}
    />
  );
}
