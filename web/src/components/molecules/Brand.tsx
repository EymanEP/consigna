import { Mono } from '../atoms';
import styles from './Brand.module.css';

interface BrandProps {
  size?: 'lg' | 'sm';
  /** "LOCAL SESSION // NO INTERNET // 9FQ2XK" */
  subline?: string;
}

export function Brand({ size = 'lg', subline }: BrandProps) {
  return (
    <div className={[styles.brand, size === 'sm' && styles.small].filter(Boolean).join(' ')}>
      <div className={styles.mark}>
        <span className={styles.bar} aria-hidden="true" />
        <span className={styles.word}>CONSIGNA</span>
      </div>
      {subline && <Mono tone="dim">{subline}</Mono>}
    </div>
  );
}
