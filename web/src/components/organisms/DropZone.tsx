import { useId, useRef, useState, type DragEvent } from 'react';

import { plural } from '../../lib/format';
import { Button, Corners, Icon, Mono, PrepBars } from '../atoms';
import styles from './DropZone.module.css';

interface DropZoneProps {
  variant: 'hero' | 'compact' | 'desktop';
  /** Files handed over but not yet started (shows "Preparing…"). */
  preparing: number;
  label?: string;
  onFiles: (files: File[]) => void;
}

/**
 * Where files go in: tap to pick, or drop them here on a computer. The file
 * input is a real control, so it works with keyboards and screen readers.
 */
export function DropZone({ variant, preparing, label, onFiles }: DropZoneProps) {
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const depth = useRef(0);
  const hintId = useId();

  const pick = () => input.current?.click();

  const onDrag = (e: DragEvent) => {
    if (!e.dataTransfer.types.includes('Files')) return;
    e.preventDefault();
    if (e.type === 'dragenter') depth.current++;
    if (e.type === 'dragleave') depth.current--;
    setOver(depth.current > 0);
  };

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    depth.current = 0;
    setOver(false);
    onFiles(Array.from(e.dataTransfer.files));
  };

  const fileInput = (
    <input
      ref={input}
      type="file"
      multiple
      hidden
      onChange={(e) => {
        onFiles(Array.from(e.target.files ?? []));
        e.target.value = '';
      }}
    />
  );

  const dragProps = { onDragEnter: onDrag, onDragOver: onDrag, onDragLeave: onDrag, onDrop };

  if (preparing > 0 && variant !== 'compact') {
    return (
      <div
        className={`${styles.zone} ${variant === 'desktop' ? styles.large : styles.hero} ${styles.preparing}`}
        role="status"
      >
        <PrepBars />
        <span className={styles.title} style={{ fontSize: 'var(--text-lead)' }}>
          Preparing {plural(preparing, 'file')}…
        </span>
        <p className={styles.body}>Large videos can take a few seconds to hand over. No need to tap again.</p>
        {fileInput}
      </div>
    );
  }

  if (variant === 'compact') {
    return (
      <div className={`${styles.zone} ${styles.compact} ${over ? styles.over : ''}`} {...dragProps}>
        <button type="button" className={styles.trigger} onClick={pick} aria-label={label ?? 'Add files'} />
        <span className={styles.content}>
          {preparing > 0 ? (
            <PrepBars small />
          ) : (
            <Icon name="add" size={18} strokeWidth={1.5} className={styles.glyph} />
          )}
          {preparing > 0 ? `Preparing ${plural(preparing, 'file')}…` : (label ?? 'Tap to add files')}
        </span>
        {fileInput}
      </div>
    );
  }

  const desktop = variant === 'desktop';
  return (
    <div
      className={`${styles.zone} ${desktop ? styles.large : styles.hero} ${over ? styles.over : ''}`}
      {...dragProps}
    >
      <Corners />
      {!desktop && (
        <button
          type="button"
          className={styles.trigger}
          onClick={pick}
          aria-label="Add files"
          aria-describedby={hintId}
        />
      )}
      <span className={styles.content}>
        <Icon
          name="drop"
          size={desktop ? 54 : 46}
          strokeWidth={desktop ? 1.3 : 1.4}
          className={styles.glyph}
        />
        <span className={styles.title}>
          {over ? 'Let go to share' : desktop ? 'Drop files here' : 'Tap to add files'}
        </span>
        <span id={hintId} className={styles.body}>
          They land in the shared tray. Any device in this session can take them.
        </span>
      </span>
      {desktop && (
        <>
          <Button variant="secondary" className={styles.browse} onClick={pick}>
            Browse files
          </Button>
          <Mono tone="dim">
            {/Mac|iPhone|iPad/.test(navigator.userAgent) ? '⌘V' : 'Ctrl+V'} pastes what is on your clipboard
          </Mono>
        </>
      )}
      {fileInput}
    </div>
  );
}
