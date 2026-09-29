import { useState } from 'react';

import { api } from '../../lib/api';
import { formatBytes, middleTruncate } from '../../lib/format';
import type { Device, HostUpload } from '../../lib/types';
import { Button, Mono, ProgressBar } from '../atoms';
import { ConfirmDialog, SectionHeader } from '../molecules';
import styles from './HostPanels.module.css';

/** What to try when a phone cannot open the link. */
export function ConnectHelp() {
  return (
    <section className={styles.panel} aria-labelledby="help-title">
      <SectionHeader id="help-title" title="Phone can't connect?" />
      <ol className={styles.steps}>
        <li>Check both devices are on the same Wi-Fi network, not one on mobile data.</li>
        <li>
          Guest networks, hotel and office Wi-Fi often stop devices from seeing each other (called AP or
          client isolation). Use your home network, or turn on the phone&apos;s hotspot and connect this
          computer to it.
        </li>
        <li>
          If your computer asked whether to allow Consigna through the firewall, allow it on private networks.
        </li>
        <li>If this computer is on several networks, pick the one the phone is on above.</li>
      </ol>
    </section>
  );
}

/** Uploads still arriving, with how far along they are. */
export function IncomingUploads({ uploads, devices }: { uploads: HostUpload[]; devices: Device[] }) {
  if (uploads.length === 0) return null;
  const names = new Map(devices.map((d) => [d.id, d.name]));
  return (
    <section className={styles.panel} aria-labelledby="incoming-title">
      <SectionHeader id="incoming-title" title="Arriving" meta={String(uploads.length)} />
      <div className={styles.list}>
        {uploads.map((u) => (
          <div key={u.id} className={styles.upload}>
            <div className={styles.uploadTop}>
              <span title={u.name}>{middleTruncate(u.name, 36)}</span>
              <Mono tone="soft" data>
                {names.get(u.ownerId) ?? 'a device'}
              </Mono>
            </div>
            <ProgressBar thin value={u.size ? u.offset / u.size : 1} label={`Receiving ${u.name}`} />
            <Mono tone="dim" data>
              {formatBytes(u.offset)} of {formatBytes(u.size)} confirmed
            </Mono>
          </div>
        ))}
      </div>
    </section>
  );
}

/** Ends the session: every file deleted, every device signed out. */
export function EndSession() {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const end = async () => {
    setBusy(true);
    try {
      await api.host.endSession();
      setOpen(false);
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className={styles.panel} aria-labelledby="end-title">
      <SectionHeader id="end-title" title="End session" />
      <p className={styles.text}>
        Deletes every file in the tray and signs every device out. A new code is made for the next session.
      </p>
      <div>
        <Button variant="danger" onClick={() => setOpen(true)}>
          End session
        </Button>
      </div>
      <ConfirmDialog
        open={open}
        title="End the session?"
        confirmLabel={busy ? 'Ending…' : 'End and delete all'}
        busy={busy}
        onConfirm={() => void end()}
        onCancel={() => setOpen(false)}
      >
        Every file in the tray is deleted and every device is signed out. This cannot be undone.
      </ConfirmDialog>
    </section>
  );
}
