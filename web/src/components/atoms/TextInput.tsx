import type { ComponentProps } from 'react';

import styles from './TextInput.module.css';

export function TextInput({ className, ...rest }: ComponentProps<'input'>) {
  return <input className={[styles.input, className].filter(Boolean).join(' ')} {...rest} />;
}

export function Select({ className, ...rest }: ComponentProps<'select'>) {
  return <select className={[styles.input, styles.select, className].filter(Boolean).join(' ')} {...rest} />;
}
