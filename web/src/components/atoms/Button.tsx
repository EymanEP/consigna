import type { ButtonHTMLAttributes, ReactNode } from 'react';

import styles from './Button.module.css';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'quiet' | 'danger' | 'warning';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: 'md' | 'lg';
  block?: boolean;
  children: ReactNode;
}

/** The mono, upper-case command button used across the CRT interface. */
export function Button({
  variant = 'secondary',
  size = 'md',
  block = false,
  className,
  type,
  ...rest
}: ButtonProps) {
  const cls = [styles.button, styles[variant], size === 'lg' && styles.lg, block && styles.block, className]
    .filter(Boolean)
    .join(' ');
  return <button type={type ?? 'button'} className={cls} {...rest} />;
}
