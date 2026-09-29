import { useEffect, useState } from 'react';

import { copyText } from '../../lib/clipboard';
import { Button, Mono } from '../atoms';
import styles from './CopyField.module.css';

interface CopyFieldProps {
  label: string;
  value: string;
}

/** A labelled value with a COPY button: "GET ANOTHER DEVICE IN". */
export function CopyField({ label, value }: CopyFieldProps) {
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle');

  useEffect(() => {
    if (state === 'idle') return;
    const t = setTimeout(() => setState('idle'), 2000);
    return () => clearTimeout(t);
  }, [state]);

  const copy = async () => setState((await copyText(value)) ? 'copied' : 'failed');

  return (
    <div className={styles.box}>
      <Mono tone="dim">{label}</Mono>
      <div className={styles.row}>
        <code className={styles.value}>{value}</code>
        <Button onClick={() => void copy()} aria-live="polite">
          {state === 'copied' ? 'Copied' : state === 'failed' ? 'Select it' : 'Copy'}
        </Button>
      </div>
    </div>
  );
}
