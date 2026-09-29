import { useState, type ReactNode } from 'react';

import { DESKTOP_QUERY, useMediaQuery } from '../hooks/useMediaQuery';
import { useNow } from '../hooks/useNow';
import { api } from '../lib/api';
import type { TrayState } from '../lib/types';
import { session, type SessionState } from '../state/session';
import { AppLink, Mono } from '../components/atoms';
import { Notice, StorageMeter, tabPanelProps, Tabs } from '../components/molecules';
import {
  AppHeader,
  ConnectHelp,
  ConnectionBanner,
  DevicesPanel,
  EndSession,
  IncomingUploads,
  InvitePanel,
  SettingsPanel,
} from '../components/organisms';
import { CenteredLayout, MobileLayout, WideLayout } from '../components/templates';

type TabKey = 'session' | 'devices' | 'settings';

async function rename(name: string) {
  await api.rename(name);
  session.refresh();
}

/** The host's control room: invite devices, see who is in, change limits. */
export function HostPage({ tray, state }: { tray: TrayState; state: SessionState }) {
  const desktop = useMediaQuery(DESKTOP_QUERY);
  const now = useNow(15_000, state.clockOffset);
  const [tab, setTab] = useState<TabKey>('session');
  const host = tray.host;

  if (!host) {
    return (
      <CenteredLayout>
        <Notice
          tone="warning"
          title="Only on the host computer"
          action={<AppLink href="/">Back to the tray</AppLink>}
        >
          The host view opens on the computer running Consigna, at http://localhost. From other devices, use
          the tray.
        </Notice>
      </CenteredLayout>
    );
  }

  const online = tray.devices.filter((d) => d.online).length;
  const tabs = [
    { key: 'session', label: 'Session' },
    { key: 'devices', label: `Devices · ${String(online)}` },
    { key: 'settings', label: 'Settings' },
  ] as const;

  const header = (
    <AppHeader
      me={tray.me}
      devices={tray.devices}
      code={tray.session.code}
      connection={state.connection}
      layout={desktop ? 'wide' : 'compact'}
      link={{ href: '/', label: 'Open tray' }}
      onRename={rename}
    />
  );

  const panels: Record<TabKey, ReactNode> = {
    session: (
      <div
        style={{
          display: 'grid',
          gap: 32,
          gridTemplateColumns: desktop ? 'minmax(0, 1.5fr) minmax(0, 1fr)' : '1fr',
        }}
      >
        <InvitePanel addresses={host.addresses} code={tray.session.code} />
        <div style={{ display: 'flex', flexDirection: 'column', gap: 28 }}>
          <StorageMeter storage={tray.storage} ttlSeconds={tray.settings.fileTtlSeconds} />
          <IncomingUploads uploads={host.uploads} devices={tray.devices} />
          <ConnectHelp />
        </div>
      </div>
    ),
    devices: <DevicesPanel devices={tray.devices} now={now} />,
    settings: (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 40 }}>
        <SettingsPanel settings={tray.settings} storage={tray.storage} />
        <EndSession />
      </div>
    ),
  };

  const body = (
    <>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, flexWrap: 'wrap' }}>
        <h1 style={{ fontSize: 24, fontWeight: 800, letterSpacing: '0.08em' }}>HOST VIEW</h1>
        <Mono tone="dim">
          This computer only · {tray.devices.length} in session ·{' '}
          {/^\d/.test(host.version) ? `v${host.version}` : host.version}
        </Mono>
      </div>
      <Tabs items={tabs} active={tab} onChange={setTab} label="Host sections" idPrefix="host" />
      <div {...tabPanelProps('host', tab)}>{panels[tab]}</div>
    </>
  );

  const banner = (
    <ConnectionBanner visible={state.connection === 'reconnecting' || state.connection === 'offline'} />
  );

  if (desktop) {
    return (
      <>
        {banner}
        <WideLayout header={header}>{body}</WideLayout>
      </>
    );
  }
  return (
    <>
      {banner}
      <MobileLayout>
        {header}
        {body}
      </MobileLayout>
    </>
  );
}
