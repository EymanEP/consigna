import { createPortal } from 'react-dom';

import { PrepBars } from '../atoms';
import styles from './ConnectionBanner.module.css';

/** Shown while the live connection to the host is down. */
export function ConnectionBanner({ visible }: { visible: boolean }) {
  return createPortal(
    <div role="status" aria-live="polite">
      {visible && (
        <div className={styles.banner}>
          <PrepBars small />
          Reconnecting to the host · transfers resume on their own
        </div>
      )}
    </div>,
    document.body,
  );
}
