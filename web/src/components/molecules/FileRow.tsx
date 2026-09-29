import { urls } from '../../lib/api';
import { formatAge, formatBytes, formatRemaining, middleTruncate } from '../../lib/format';
import type { TrayFile } from '../../lib/types';
import { Checkbox, IconLink, Mono, TypeBadge } from '../atoms';
import { ConfirmButton } from './ConfirmButton';
import styles from './FileRow.module.css';

export interface FileRowProps {
  file: TrayFile;
  now: number;
  layout: 'compact' | 'wide';
  isNew?: boolean;
  selecting?: boolean;
  selected?: boolean;
  onToggle?: (id: string) => void;
  onDelete: (file: TrayFile) => void;
  onDownload: (file: TrayFile) => void;
}

/** One file in the shared tray. */
export function FileRow({
  file,
  now,
  layout,
  isNew = false,
  selecting = false,
  selected = false,
  onToggle,
  onDelete,
  onDownload,
}: FileRowProps) {
  const size = formatBytes(file.size);
  const age = formatAge(file.addedAt, now);
  const expires = `Added by ${file.uploaderName}. Disappears in ${formatRemaining(file.expiresAt, now)}.`;
  const rowCls = [
    styles.row,
    layout === 'wide' && styles.wide,
    isNew && styles.new,
    selected && styles.selected,
    selecting && styles.selectable,
  ]
    .filter(Boolean)
    .join(' ');

  const lead = selecting ? (
    <Checkbox label={`Select ${file.name}`} checked={selected} onChange={() => onToggle?.(file.id)} />
  ) : (
    <TypeBadge name={file.name} />
  );
  const newTag = isNew ? (
    <Mono tone="accent" className={styles.tag}>
      New
    </Mono>
  ) : null;
  const del = (
    <ConfirmButton label={`Delete ${file.name}`} onConfirm={() => onDelete(file)} className={styles.del}>
      Del
    </ConfirmButton>
  );

  if (layout === 'wide') {
    return (
      <div className={rowCls} title={expires}>
        {lead}
        <span className={styles.cell} title={file.name}>
          {middleTruncate(file.name, 48)}
          {newTag}
        </span>
        <Mono tone="soft" data className={styles.cell}>
          {size}
        </Mono>
        <Mono tone="soft" data className={styles.cell}>
          {file.uploaderName}
        </Mono>
        <Mono tone="soft" data className={styles.cell}>
          {age}
        </Mono>
        <span className={styles.actions}>
          {!selecting && (
            <>
              {del}
              <a
                className={styles.getLink}
                href={urls.file(file.id)}
                download={file.name}
                aria-label={`Download ${file.name}`}
                onClick={() => onDownload(file)}
              >
                GET
              </a>
            </>
          )}
        </span>
      </div>
    );
  }

  return (
    <div className={rowCls}>
      {lead}
      <div className={styles.main}>
        <div className={styles.name} title={file.name}>
          {middleTruncate(file.name, 30)}
          {newTag}
        </div>
        <div className={styles.meta}>
          <Mono tone="dim" data title={expires}>
            {size} · {file.uploaderName} · {age}
          </Mono>
          {!selecting && del}
        </div>
      </div>
      {!selecting && (
        <IconLink
          icon="download"
          variant="lit"
          label={`Download ${file.name}`}
          href={urls.file(file.id)}
          download={file.name}
          onClick={() => onDownload(file)}
        />
      )}
    </div>
  );
}
