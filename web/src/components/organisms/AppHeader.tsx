import { plural } from '../../lib/format';
import { useStore } from '../../lib/store';
import type { Device } from '../../lib/types';
import type { Connection } from '../../state/session';
import { fx, setFx } from '../../state/fx';
import { AppLink, Icon, IconButton, Mono, StatusDot } from '../atoms';
import { DeviceName } from '../molecules';
import styles from './AppHeader.module.css';

interface AppHeaderProps {
  me: Device;
  devices: Device[];
  code: string;
  connection: Connection;
  layout: 'compact' | 'wide';
  /** Where the nav link points: the host view, or back to the tray. */
  link?: { href: string; label: string } | null;
  onRename: (name: string) => Promise<void>;
}

/** Live dot, this device's name, who else is here, and quick controls. */
export function AppHeader({ me, devices, code, connection, layout, link, onRename }: AppHeaderProps) {
  const level = useStore(fx);
  const others = devices.filter((d) => !d.isMe && d.online);
  const live = connection === 'live';
  const dot = <StatusDot state={live ? 'live' : 'paused'} pulse />;
  const fxToggle = (
    <IconButton
      icon={level === 'calm' ? 'spark' : 'monitor'}
      label={level === 'calm' ? 'Turn screen effects on' : 'Calm the screen effects'}
      aria-pressed={level === 'calm'}
      onClick={() => setFx(level === 'calm' ? 'full' : 'calm')}
    />
  );
  const nav = link && (
    <AppLink href={link.href} className={styles.navLink}>
      {link.label}
      <Icon name="arrowRight" size={14} />
    </AppLink>
  );
  const connectionText = live ? null : <span className="visually-hidden">Reconnecting to the host.</span>;

  if (layout === 'wide') {
    return (
      <header className={styles.header}>
        <div className={styles.mark}>
          <span className={styles.markBar} aria-hidden="true" />
          <span className={styles.markWord}>CONSIGNA</span>
        </div>
        {dot}
        {connectionText}
        <DeviceName name={me.name} onRename={onRename} />
        <Mono tone="muted" className={styles.others}>
          {others.length === 0
            ? 'No other devices yet'
            : `${plural(others.length, 'other device')} · ${others.map((d) => d.name).join(', ')}`}
        </Mono>
        <span className={styles.spacer} />
        <Mono tone="dim">Session {code}</Mono>
        <nav className={styles.nav} aria-label="Views">
          {nav}
        </nav>
        {fxToggle}
      </header>
    );
  }

  return (
    <header className={styles.header}>
      {dot}
      {connectionText}
      <DeviceName name={me.name} onRename={onRename} />
      <span className={styles.spacer} />
      <Mono tone="muted">{plural(others.length + 1, 'device')}</Mono>
      {nav && <nav aria-label="Views">{nav}</nav>}
      {fxToggle}
    </header>
  );
}
