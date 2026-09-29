import { formatBytes, formatEta, formatRate, middleTruncate } from '../../lib/format';
import type { Transfer } from '../../state/uploads';
import { Button, Icon, IconButton, Mono, PrepBars, ProgressBar } from '../atoms';
import styles from './TransferCard.module.css';

interface TransferCardProps {
  transfer: Transfer;
  /** Show the "keep this tab open" hint (phones suspend background tabs). */
  showKeepOpen: boolean;
  onResume: (id: string) => void;
  onCancel: (id: string) => void;
  onRetry: (id: string) => void;
  onDismiss: (id: string) => void;
}

/** The transfer being sent right now, including its paused and failed states. */
export function TransferCard({
  transfer: t,
  showKeepOpen,
  onResume,
  onCancel,
  onRetry,
  onDismiss,
}: TransferCardProps) {
  const share = t.size > 0 ? t.sent / t.size : t.status === 'done' ? 1 : 0;
  const pct = Math.floor(share * 100);
  const paused = t.status === 'paused';
  const failed = t.status === 'failed';
  const done = t.status === 'done';
  const progress = `${formatBytes(t.sent)} of ${formatBytes(t.size)}`;

  let meta = progress;
  if (t.status === 'uploading' && t.rate > 0) {
    meta += ` · ${formatRate(t.rate)} · ${formatEta((t.size - t.sent) / t.rate)}`;
  } else if (paused) {
    meta += ' · stopped';
  } else if (t.status === 'preparing' || (t.status === 'uploading' && t.sent === 0)) {
    meta = `${formatBytes(t.size)} · starting`;
  } else if (done) {
    meta = `${formatBytes(t.size)} · in the tray`;
  }

  const cardCls = [styles.card, paused && styles.pausedCard, failed && styles.failedCard]
    .filter(Boolean)
    .join(' ');
  const pctCls = [styles.pct, paused && styles.pausedText, failed && styles.failedText]
    .filter(Boolean)
    .join(' ');

  return (
    <article className={cardCls} aria-label={`Sending ${t.name}`}>
      <div className={styles.top}>
        <Icon name="up" size={15} strokeWidth={2} className={styles.arrow} />
        <div className={styles.main}>
          <div className={styles.name} title={t.name}>
            {middleTruncate(t.name, 34)}
          </div>
          <Mono tone="soft" data className={styles.meta}>
            {meta}
          </Mono>
        </div>
        <div className={pctCls} aria-hidden="true">
          {t.status === 'preparing' ? (
            <PrepBars small />
          ) : done ? (
            <Icon name="check" size={24} />
          ) : (
            `${String(pct)}%`
          )}
        </div>
      </div>

      <div className={styles.bar}>
        <ProgressBar
          value={share}
          tone={paused ? 'paused' : failed ? 'failed' : 'active'}
          label={`Sending ${t.name}`}
          valueText={`${String(pct)}%, ${progress}${paused ? ', paused' : ''}`}
        />
      </div>

      {(t.status === 'uploading' || t.status === 'preparing') && (
        <div className={styles.foot}>
          <Icon name="monitor" size={14} strokeWidth={1.6} />
          <span className={styles.hint}>
            {showKeepOpen
              ? 'Keep this tab open until it finishes.'
              : 'You can keep working; it runs in the background.'}
          </span>
          <IconButton
            icon="close"
            label={`Cancel sending ${t.name}`}
            className={styles.cancel}
            onClick={() => onCancel(t.id)}
          />
        </div>
      )}

      {paused && (
        <div className={styles.status} role="status">
          <p className={`${styles.statusTitle} ${styles.pausedText}`}>
            Paused — {formatBytes(t.sent)} is already across.
          </p>
          <p className={styles.statusBody}>
            Nothing is lost. It carries on from where it stopped, on its own when the connection is back.
          </p>
          <div className={styles.actions}>
            <Button variant="warning" size="lg" onClick={() => onResume(t.id)}>
              Tap to resume
            </Button>
          </div>
        </div>
      )}

      {failed && (
        <div className={styles.status} role="alert">
          <p className={`${styles.statusTitle} ${styles.failedText}`}>Could not send this file.</p>
          <p className={styles.statusBody}>{t.message}</p>
          <div className={styles.actions}>
            <Button variant="secondary" onClick={() => onRetry(t.id)}>
              Try again
            </Button>
            <Button variant="ghost" onClick={() => onDismiss(t.id)}>
              Dismiss
            </Button>
          </div>
        </div>
      )}
    </article>
  );
}
