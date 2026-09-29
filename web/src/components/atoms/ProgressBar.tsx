import styles from './ProgressBar.module.css';

interface ProgressBarProps {
  /** 0 to 1. */
  value: number;
  tone?: 'active' | 'paused' | 'failed';
  thin?: boolean;
  /** Accessible name, e.g. "Upload progress". */
  label: string;
  /** Human text for screen readers, e.g. "34%, 412 MB of 1.2 GB". */
  valueText?: string;
}

export function ProgressBar({ value, tone = 'active', thin = false, label, valueText }: ProgressBarProps) {
  const pct = Math.round(Math.min(1, Math.max(0, value)) * 1000) / 10;
  const cls = [styles.track, tone !== 'active' && styles[tone], thin && styles.thin]
    .filter(Boolean)
    .join(' ');
  return (
    <div
      className={cls}
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(pct)}
      aria-valuetext={valueText}
    >
      <div className={styles.fill} style={{ width: `${String(pct)}%` }} />
      {!thin && <div className={styles.head} style={{ left: `${String(pct)}%` }} />}
    </div>
  );
}
