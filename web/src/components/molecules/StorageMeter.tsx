import { formatBytes, formatDuration } from '../../lib/format';
import type { Storage } from '../../lib/types';
import { Mono, ProgressBar } from '../atoms';
import styles from './StorageMeter.module.css';

/** Share of the tray at which it counts as nearly full. */
export const NEARLY_FULL = 0.9;

interface StorageMeterProps {
  storage: Storage;
  ttlSeconds?: number;
}

/** "TRAY  2.4 GB / 10 GB" and a thin meter; amber when nearly full. */
export function StorageMeter({ storage, ttlSeconds }: StorageMeterProps) {
  const used = storage.used + storage.reserved;
  const share = storage.limit > 0 ? used / storage.limit : 0;
  const full = share >= NEARLY_FULL;
  return (
    <div className={styles.meter}>
      <div className={styles.labels}>
        <Mono tone={full ? 'paused' : 'dim'}>{full ? 'Tray nearly full' : 'Tray'}</Mono>
        <Mono tone={full ? 'paused' : 'dim'}>
          {formatBytes(used)} / {formatBytes(storage.limit)}
        </Mono>
      </div>
      <ProgressBar
        thin
        value={share}
        tone={full ? 'paused' : 'active'}
        label="Tray storage used"
        valueText={`${formatBytes(used)} of ${formatBytes(storage.limit)}`}
      />
      {ttlSeconds !== undefined && (
        <p className={styles.note}>Files disappear {formatDuration(ttlSeconds)} after they arrive.</p>
      )}
    </div>
  );
}
