import { api } from '../../lib/api';
import { plural } from '../../lib/format';
import type { Device } from '../../lib/types';
import { DeviceRow, SectionHeader } from '../molecules';
import styles from './HostPanels.module.css';

/** Every device in the session with its IP address; the host can remove any. */
export function DevicesPanel({ devices, now }: { devices: Device[]; now: number }) {
  const online = devices.filter((d) => d.online).length;
  const sorted = [...devices].sort(
    (a, b) => Number(b.online) - Number(a.online) || a.name.localeCompare(b.name),
  );
  return (
    <section className={styles.panel} aria-labelledby="devices-title">
      <SectionHeader
        id="devices-title"
        title="Devices"
        meta={`${String(online)} online · ${plural(devices.length, 'device')} total`}
        lit
      />
      <p className={styles.text}>
        Names are made up at random when a device joins; people can rename their own. Removing a device signs
        it out and stops its uploads. It can come back with the current QR code or session code.
      </p>
      <div className={styles.list} role="list" aria-label="Devices in the session">
        {sorted.map((d) => (
          <div role="listitem" key={d.id}>
            <DeviceRow
              device={d}
              now={now}
              onRemove={(dev) => void api.host.removeDevice(dev.id).catch(() => undefined)}
            />
          </div>
        ))}
      </div>
    </section>
  );
}
