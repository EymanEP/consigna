import { useEffect, useRef } from 'react';

import type { Transfer } from '../../state/uploads';
import { QueuedRow, SectionHeader, TransferCard } from '../molecules';
import { useCrt } from '../templates';
import styles from './TransferPanel.module.css';

interface TransferPanelProps {
  transfers: readonly Transfer[];
  showKeepOpen: boolean;
  onResume: (id: string) => void;
  onCancel: (id: string) => void;
  onRetry: (id: string) => void;
  onDismiss: (id: string) => void;
}

/** "ACTIVE TRANSFERS": the file being sent, anything that failed, and the queue. */
export function TransferPanel({ transfers, showKeepOpen, ...actions }: TransferPanelProps) {
  const { glitch } = useCrt();
  const cards = transfers.filter(
    (t) => t.status === 'uploading' || t.status === 'paused' || t.status === 'failed' || t.status === 'done',
  );
  const waiting = transfers.filter((t) => t.status === 'queued' || t.status === 'preparing');
  const paused = transfers.filter((t) => t.status === 'paused').length;
  const pending = transfers.filter((t) => t.status !== 'done' && t.status !== 'failed');
  const position = transfers.length - pending.length + 1;

  // Pausing and resuming are felt, not just read: a heavier one-off glitch.
  const wasPaused = useRef(paused);
  useEffect(() => {
    if (paused !== wasPaused.current) glitch();
    wasPaused.current = paused;
  }, [paused, glitch]);

  const meta =
    paused > 0
      ? `${String(paused)} paused${waiting.length ? ` · ${String(waiting.length)} waiting` : ''}`
      : pending.length > 0
        ? `${String(Math.min(position, transfers.length))} of ${String(transfers.length)}`
        : undefined;

  return (
    <section className={styles.panel} aria-labelledby="transfers-title">
      <SectionHeader id="transfers-title" title="Active transfers" meta={meta} lit />
      {cards.map((t) => (
        <TransferCard key={t.id} transfer={t} showKeepOpen={showKeepOpen} {...actions} />
      ))}
      {waiting.length > 0 && (
        <div className={styles.queue}>
          {waiting.map((t) => (
            <QueuedRow key={t.id} transfer={t} onCancel={actions.onCancel} />
          ))}
        </div>
      )}
    </section>
  );
}
