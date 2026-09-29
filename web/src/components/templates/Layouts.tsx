import type { ReactNode } from 'react';

import styles from './Layouts.module.css';

/** Phone layout: one column, the page scrolls. */
export function MobileLayout({ children }: { children: ReactNode }) {
  return <main className={styles.mobile}>{children}</main>;
}

/** Desktop layout: sending on the left, the tray on the right. */
export function DesktopLayout({
  header,
  left,
  right,
}: {
  header: ReactNode;
  left: ReactNode;
  right: ReactNode;
}) {
  return (
    <div className={styles.desktop}>
      {header}
      <main className={styles.columns}>
        <div className={styles.column}>{left}</div>
        <div className={styles.column}>{right}</div>
      </main>
    </div>
  );
}

/** Desktop layout with one full-width column, for the host view. */
export function WideLayout({ header, children }: { header: ReactNode; children: ReactNode }) {
  return (
    <div className={styles.desktop}>
      {header}
      <main className={styles.wide}>{children}</main>
    </div>
  );
}

/** A narrow, vertically centred column for join, loading and ended screens. */
export function CenteredLayout({ children }: { children: ReactNode }) {
  return <main className={styles.centered}>{children}</main>;
}

/** Pushes what follows to the bottom of a flex column. */
export function Spacer() {
  return <div className={styles.spacer} aria-hidden="true" />;
}
