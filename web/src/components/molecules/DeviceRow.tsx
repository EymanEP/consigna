import { formatAge } from '../../lib/format';
import type { Device } from '../../lib/types';
import { Icon, Mono, StatusDot, type IconName } from '../atoms';
import { ConfirmButton } from './ConfirmButton';
import styles from './DeviceRow.module.css';

const kindIcon: Record<Device['kind'], IconName> = {
  phone: 'phone',
  tablet: 'tablet',
  desktop: 'laptop',
  unknown: 'monitor',
};

interface DeviceRowProps {
  device: Device;
  now: number;
  onRemove: (device: Device) => void;
}

/** A device in the host's device list, with its IP address. */
export function DeviceRow({ device: d, now, onRemove }: DeviceRowProps) {
  const tags = [d.isMe && 'this browser', d.isHost && !d.isMe && 'host'].filter(Boolean).join(' · ');
  return (
    <div className={styles.row}>
      <Icon name={kindIcon[d.kind]} className={styles.icon} />
      <div className={styles.name}>
        <span className={styles.nameText}>{d.name}</span>
        {tags && <Mono tone="dim">{tags}</Mono>}
      </div>
      <Mono tone="soft" data className={styles.ip}>
        {d.ip ?? '—'}
      </Mono>
      <span className={styles.status}>
        <StatusDot state={d.online ? 'live' : 'off'} />
        <Mono tone={d.online ? 'soft' : 'dim'} data>
          {d.online
            ? 'online'
            : d.lastSeen
              ? `seen ${formatAge(d.lastSeen, now)} ago`.replace('seen now ago', 'seen now')
              : 'offline'}
        </Mono>
      </span>
      <span className={styles.action}>
        {!d.isMe && (
          <ConfirmButton
            label={`Remove ${d.name} from the session`}
            onConfirm={() => onRemove(d)}
            variant="ghost"
            confirmLabel="Remove?"
          >
            Remove
          </ConfirmButton>
        )}
      </span>
    </div>
  );
}
