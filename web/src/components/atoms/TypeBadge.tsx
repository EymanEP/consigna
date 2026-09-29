import { fileTypeLabel } from '../../lib/format';
import styles from './TypeBadge.module.css';

/** The boxed file-type label ("ZIP", "MP4"). Decorative. */
export function TypeBadge({ name }: { name: string }) {
  return (
    <span className={styles.badge} aria-hidden="true">
      {fileTypeLabel(name)}
    </span>
  );
}
