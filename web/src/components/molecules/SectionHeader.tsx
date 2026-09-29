import type { ReactNode } from 'react';

import { Mono, Rule } from '../atoms';
import styles from './SectionHeader.module.css';

interface SectionHeaderProps {
  /** Element id so a region can be labelled by this header. */
  id?: string;
  title: string;
  meta?: ReactNode;
  lit?: boolean;
  /** Buttons shown instead of the hairline. */
  actions?: ReactNode;
}

/** "ACTIVE TRANSFERS ———— 1 OF 3" */
export function SectionHeader({ id, title, meta, lit = false, actions }: SectionHeaderProps) {
  return (
    <div className={styles.header}>
      <h2 id={id} style={{ margin: 0, fontSize: 'inherit', fontWeight: 'inherit' }}>
        <Mono tone={lit ? 'accent' : 'dim'} style={{ letterSpacing: '0.2em' }}>
          {title}
        </Mono>
      </h2>
      {actions ? (
        <>
          {meta !== undefined && <Mono tone="dim">{meta}</Mono>}
          <div className={styles.actions}>{actions}</div>
        </>
      ) : (
        <>
          <Rule />
          {meta !== undefined && <Mono tone="dim">{meta}</Mono>}
        </>
      )}
    </div>
  );
}
