import { useState } from 'react';

import { api, urls } from '../../lib/api';
import type { Address } from '../../lib/types';
import { Button, Corners, Icon, Mono, type IconName } from '../atoms';
import { CopyField, Notice, SectionHeader } from '../molecules';
import styles from './HostPanels.module.css';

const kindLabel: Record<Address['kind'], { label: string; icon: IconName }> = {
  wifi: { label: 'Wi-Fi', icon: 'wifi' },
  ethernet: { label: 'Ethernet', icon: 'ethernet' },
  vpn: { label: 'VPN', icon: 'vpn' },
  virtual: { label: 'Virtual', icon: 'monitor' },
  other: { label: 'Network', icon: 'wifi' },
};

interface InvitePanelProps {
  addresses: Address[];
  code: string;
}

/** The host's QR code, join link, network picker and join code. */
export function InvitePanel({ addresses, code }: InvitePanelProps) {
  const [ip, setIp] = useState(addresses[0]?.ip ?? '');
  const [rotating, setRotating] = useState(false);

  // Falls back to the best address if the chosen one disappears.
  const current = addresses.find((a) => a.ip === ip) ?? addresses[0];

  const rotate = async () => {
    setRotating(true);
    try {
      await api.host.rotateCode();
    } finally {
      setRotating(false);
    }
  };

  if (!current) {
    return (
      <Notice tone="warning" title="No network connection" live>
        This computer is not on a network. Connect it to Wi-Fi or Ethernet; the QR code appears here as soon
        as it is.
      </Notice>
    );
  }

  return (
    <section className={styles.panel} aria-labelledby="invite-title">
      <SectionHeader id="invite-title" title="Bring a device in" lit />
      <div className={styles.qrRow}>
        <div className={styles.qrFrame}>
          <Corners />
          {/* The code changes when the join code rotates; the key forces a reload. */}
          <img
            key={`${current.ip}-${code}`}
            className={styles.qr}
            src={urls.qr(current.ip)}
            alt={`QR code for ${current.url}`}
          />
        </div>
        <div className={styles.qrSide}>
          <p className={styles.lead}>Scan with the phone&apos;s camera.</p>
          <p className={styles.text}>
            The device must be on the same Wi-Fi as this computer. It joins as soon as the page opens; nothing
            to install.
          </p>
          <CopyField label="Or open this address" value={current.url} />
          <div className={styles.code}>
            <div>
              <Mono tone="dim">Session code</Mono>
              <div className={styles.codeValue}>{code}</div>
            </div>
            <Button
              onClick={() => void rotate()}
              disabled={rotating}
              title="Old links and QR codes stop working; devices already in stay in."
            >
              <Icon name="refresh" size={14} />
              New code
            </Button>
          </div>
        </div>
      </div>

      {addresses.length > 1 && (
        <fieldset className={styles.addresses}>
          <legend style={{ marginBottom: 8 }}>
            <Mono tone="dim">This computer is on several networks. Use the one the other device is on:</Mono>
          </legend>
          {addresses.map((a) => (
            <label key={a.ip} className={styles.address}>
              <input
                type="radio"
                name="address"
                value={a.ip}
                checked={a.ip === current.ip}
                onChange={() => setIp(a.ip)}
              />
              <Icon name={kindLabel[a.kind].icon} size={16} />
              <span className={styles.addressName}>
                {kindLabel[a.kind].label} · {a.interface}
              </span>
              <Mono tone="soft" data>
                {a.ip}
              </Mono>
            </label>
          ))}
        </fieldset>
      )}
    </section>
  );
}
