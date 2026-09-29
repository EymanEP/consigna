import type { ReactNode } from 'react';

import { Icon, type IconName } from '../atoms';
import styles from './Notice.module.css';

interface NoticeProps {
  tone?: 'info' | 'warning' | 'danger';
  icon?: IconName;
  title: string;
  children?: ReactNode;
  action?: ReactNode;
  /** Announce to screen readers when it appears. */
  live?: boolean;
}

export function Notice({ tone = 'info', icon, title, children, action, live = false }: NoticeProps) {
  const cls = [styles.notice, tone !== 'info' && styles[tone]].filter(Boolean).join(' ');
  return (
    <div className={cls} role={live ? (tone === 'danger' ? 'alert' : 'status') : undefined}>
      <Icon name={icon ?? (tone === 'info' ? 'spark' : 'warning')} className={styles.icon} />
      <div className={styles.body}>
        <p className={styles.title}>{title}</p>
        {children && <div className={styles.text}>{children}</div>}
        {action && <div className={styles.action}>{action}</div>}
      </div>
    </div>
  );
}
