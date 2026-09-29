import { formatBytes, middleTruncate } from '../../lib/format';
import type { Transfer } from '../../state/uploads';
import { IconButton, Mono } from '../atoms';
import styles from './QueuedRow.module.css';

interface QueuedRowProps {
  transfer: Transfer;
  onCancel: (id: string) => void;
}

/** A file waiting its turn to be sent. */
export function QueuedRow({ transfer, onCancel }: QueuedRowProps) {
  return (
    <div className={styles.row}>
      <Mono tone="dim" className={styles.state}>
        {transfer.status === 'preparing' ? 'Reading' : 'Queued'}
      </Mono>
      <span className={styles.name} title={transfer.name}>
        {middleTruncate(transfer.name, 34)}
      </span>
      <Mono tone="dim" data>
        {formatBytes(transfer.size)}
      </Mono>
      <IconButton
        icon="close"
        label={`Remove ${transfer.name} from the queue`}
        iconSize={14}
        onClick={() => onCancel(transfer.id)}
      />
    </div>
  );
}
