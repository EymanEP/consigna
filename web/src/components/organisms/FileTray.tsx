import { useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';

import { urls } from '../../lib/api';
import { formatBytes, plural } from '../../lib/format';
import type { TrayFile } from '../../lib/types';
import { Button, Mono, PrepBars, Select } from '../atoms';
import { Chip, FileRow } from '../molecules';
import styles from './FileTray.module.css';

export type SortKey = 'newest' | 'oldest' | 'name' | 'size';

const sorters: Record<SortKey, (a: TrayFile, b: TrayFile) => number> = {
  newest: (a, b) => b.addedAt.localeCompare(a.addedAt),
  oldest: (a, b) => a.addedAt.localeCompare(b.addedAt),
  name: (a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }),
  size: (a, b) => b.size - a.size,
};

interface FileTrayProps {
  files: TrayFile[];
  now: number;
  layout: 'compact' | 'wide';
  newIds: ReadonlySet<string>;
  onSeen: (ids?: readonly string[]) => void;
  onDelete: (ids: string[]) => Promise<void>;
  /** Hide the section header while the tray is empty (the phone's first screen). */
  bareWhenEmpty?: boolean;
}

/** "SHARED FILES": everything in the tray, with sorting and multi-select. */
export function FileTray({
  files,
  now,
  layout,
  newIds,
  onSeen,
  onDelete,
  bareWhenEmpty = false,
}: FileTrayProps) {
  const [sort, setSort] = useState<SortKey>('newest');
  const [selecting, setSelecting] = useState(false);
  const [picked, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [packing, setPacking] = useState(0);

  const sorted = useMemo(() => [...files].sort(sorters[sort]), [files, sort]);
  const total = files.reduce((n, f) => n + f.size, 0);

  // Files deleted by someone else drop out of the selection.
  const selected = useMemo(() => {
    const present = new Set(files.map((f) => f.id));
    return new Set([...picked].filter((id) => present.has(id)));
  }, [files, picked]);

  useEffect(() => {
    if (packing === 0) return;
    const t = setTimeout(() => {
      setPacking(0);
      setSelecting(false);
      setSelected(new Set());
    }, 2500);
    return () => clearTimeout(t);
  }, [packing]);

  useEffect(() => {
    if (!selecting) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setSelecting(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selecting]);

  const toggle = (id: string) =>
    setSelected((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const exitSelect = () => {
    setSelecting(false);
    setSelected(new Set());
  };

  const zip = () => {
    const ids = [...selected];
    if (ids.length === 0) return;
    onSeen(ids);
    setPacking(ids.length);
    // A plain navigation lets the browser's download manager stream the ZIP.
    const a = document.createElement('a');
    a.href = urls.archive(ids);
    a.download = '';
    document.body.appendChild(a);
    a.click();
    a.remove();
  };

  const deleteSelected = async () => {
    await onDelete([...selected]);
    exitSelect();
  };

  const newCount = files.filter((f) => newIds.has(f.id)).length;

  return (
    <section className={styles.tray} aria-labelledby="tray-title">
      <div className={styles.header} hidden={bareWhenEmpty && files.length === 0}>
        <h2 id="tray-title" style={{ margin: 0, fontSize: 'inherit' }}>
          <Mono tone="accent" style={{ letterSpacing: '0.2em' }}>
            Shared files
          </Mono>
        </h2>
        <Mono tone="dim" className={styles.meta}>
          {files.length} · {formatBytes(total)}
        </Mono>
        {files.length > 0 && (
          <div className={styles.headerActions}>
            <label className="visually-hidden" htmlFor="tray-sort">
              Sort files
            </label>
            <Select
              id="tray-sort"
              className={styles.sort}
              value={sort}
              onChange={(e) => setSort(e.target.value as SortKey)}
            >
              <option value="newest">Newest</option>
              <option value="oldest">Oldest</option>
              <option value="name">Name</option>
              <option value="size">Largest</option>
            </Select>
            <Button
              variant="secondary"
              className={styles.selectBtn}
              aria-pressed={selecting}
              onClick={() => (selecting ? exitSelect() : setSelecting(true))}
            >
              {selecting ? 'Done' : 'Select'}
            </Button>
          </div>
        )}
      </div>

      {newCount > 0 && !selecting && (
        <div className={styles.chipRow}>
          <Chip onClick={() => onSeen()} aria-label={`${plural(newCount, 'new file')}. Mark as seen`}>
            {plural(newCount, 'new file')}
          </Chip>
        </div>
      )}

      {files.length === 0 ? (
        <div className={styles.empty}>
          <p className={styles.emptyTitle}>Nothing in the tray yet</p>
          <p className={styles.emptyBody}>
            You are in the session. Anything any device drops shows up here, live.
          </p>
        </div>
      ) : (
        <>
          {layout === 'wide' && (
            <div className={styles.columns} aria-hidden="true">
              <Mono tone="dim">Type</Mono>
              <Mono tone="dim">Name</Mono>
              <Mono tone="dim">Size</Mono>
              <Mono tone="dim">From</Mono>
              <Mono tone="dim">Added</Mono>
              <span />
            </div>
          )}
          <div role="list" aria-label="Files in the tray">
            {sorted.map((f) => (
              <div role="listitem" key={f.id}>
                <FileRow
                  file={f}
                  now={now}
                  layout={layout}
                  isNew={newIds.has(f.id)}
                  selecting={selecting}
                  selected={selected.has(f.id)}
                  onToggle={toggle}
                  onDelete={(file) => void onDelete([file.id])}
                  onDownload={(file) => onSeen([file.id])}
                />
              </div>
            ))}
          </div>
          {selecting && <div className={styles.selectingPad} />}
        </>
      )}

      {selecting &&
        createPortal(
          <div className={styles.selbar} role="region" aria-label="Selection">
            <div className={styles.selbarInner}>
              {packing > 0 ? (
                <div className={styles.packing} role="status">
                  <PrepBars small />
                  <div>
                    <p className={styles.packingTitle}>Packing {plural(packing, 'file')}…</p>
                    <p className={styles.packingBody}>
                      A big selection takes a moment before the download starts.
                    </p>
                  </div>
                </div>
              ) : (
                <>
                  <Mono tone="accent" className={styles.count} aria-live="polite">
                    {selected.size} selected
                  </Mono>
                  <span className={styles.selbarGrow} />
                  {layout === 'wide' && (
                    <Button
                      variant="ghost"
                      onClick={() =>
                        setSelected(
                          selected.size === files.length ? new Set() : new Set(files.map((f) => f.id)),
                        )
                      }
                    >
                      {selected.size === files.length ? 'Select none' : 'Select all'}
                    </Button>
                  )}
                  <Button variant="primary" disabled={selected.size === 0} onClick={zip}>
                    Zip
                  </Button>
                  <Button
                    variant="danger"
                    disabled={selected.size === 0}
                    onClick={() => void deleteSelected()}
                  >
                    Delete
                  </Button>
                  <Button variant="ghost" onClick={exitSelect}>
                    Cancel
                  </Button>
                </>
              )}
            </div>
          </div>,
          document.body,
        )}
    </section>
  );
}
