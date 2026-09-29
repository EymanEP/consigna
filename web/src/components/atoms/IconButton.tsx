import type { AnchorHTMLAttributes, ButtonHTMLAttributes } from 'react';

import { Icon, type IconName } from './Icon';
import styles from './IconButton.module.css';

interface Common {
  icon: IconName;
  /** Required: icon-only controls need an accessible name. */
  label: string;
  variant?: 'plain' | 'lit';
  iconSize?: number;
}

type IconButtonProps = Common & Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'>;

export function IconButton({
  icon,
  label,
  variant = 'plain',
  iconSize,
  className,
  ...rest
}: IconButtonProps) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={[styles.button, variant === 'lit' && styles.lit, className].filter(Boolean).join(' ')}
      {...rest}
    >
      <Icon name={icon} size={iconSize} />
    </button>
  );
}

type IconLinkProps = Common & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'children'>;

/** An icon-only link, e.g. a download. */
export function IconLink({ icon, label, variant = 'plain', iconSize, className, ...rest }: IconLinkProps) {
  return (
    <a
      aria-label={label}
      title={label}
      className={[styles.button, variant === 'lit' && styles.lit, className].filter(Boolean).join(' ')}
      {...rest}
    >
      <Icon name={icon} size={iconSize} />
    </a>
  );
}
