import { useCallback, useEffect } from 'react';

import { DESKTOP_QUERY, TOUCH_QUERY, useMediaQuery } from '../hooks/useMediaQuery';
import { useNow } from '../hooks/useNow';
import { ApiError, api } from '../lib/api';
import { formatBytes, plural } from '../lib/format';
import { useStore } from '../lib/store';
import type { TrayState } from '../lib/types';
import { session, type Connection, type SessionState } from '../state/session';
import { uploads } from '../state/uploads';
import { Brand, CopyField, NEARLY_FULL, Notice, StorageMeter } from '../components/molecules';
import { AppHeader, ConnectionBanner, DropZone, FileTray, TransferPanel } from '../components/organisms';
import { DesktopLayout, MobileLayout, Spacer } from '../components/templates';

interface TrayPageProps {
  tray: TrayState;
  state: SessionState;
}

async function rename(name: string) {
  await api.rename(name);
  session.refresh();
}

async function deleteFiles(ids: string[]) {
  try {
    if (ids.length === 1 && ids[0]) await api.deleteFile(ids[0]);
    else await api.deleteFiles(ids);
  } catch (err) {
    // Already gone is fine: someone else deleted it first.
    if (!(err instanceof ApiError && err.status === 404)) throw err;
  }
}

/** Files dropped or pasted anywhere on the page are shared too. */
function usePageDropAndPaste(onFiles: (files: File[]) => void) {
  useEffect(() => {
    const onDragOver = (e: DragEvent) => {
      if (e.dataTransfer?.types.includes('Files')) e.preventDefault();
    };
    const onDrop = (e: DragEvent) => {
      if (!e.dataTransfer?.files.length) return;
      // Without this the browser would navigate away to show the file.
      e.preventDefault();
      onFiles(Array.from(e.dataTransfer.files));
    };
    const onPaste = (e: ClipboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target?.closest('input, textarea, [contenteditable="true"]')) return;
      const files = Array.from(e.clipboardData?.files ?? []);
      if (files.length > 0) {
        e.preventDefault();
        onFiles(files);
      }
    };
    window.addEventListener('dragover', onDragOver);
    window.addEventListener('drop', onDrop);
    window.addEventListener('paste', onPaste);
    return () => {
      window.removeEventListener('dragover', onDragOver);
      window.removeEventListener('drop', onDrop);
      window.removeEventListener('paste', onPaste);
    };
  }, [onFiles]);
}

function StorageWarning({ tray }: { tray: TrayState }) {
  const used = tray.storage.used + tray.storage.reserved;
  if (tray.storage.limit === 0 || used / tray.storage.limit < NEARLY_FULL) return null;
  return (
    <Notice tone="warning" title="The tray is nearly full" live>
      {formatBytes(used)} of {formatBytes(tray.storage.limit)} in use. Delete files nobody needs any more, or
      ask the host to raise the limit.
    </Notice>
  );
}

function hostLink(tray: TrayState) {
  return tray.host ? { href: '/host', label: 'Host' } : null;
}

/** The shared tray: send files, see and take everyone's files. */
export function TrayPage({ tray, state }: TrayPageProps) {
  const desktop = useMediaQuery(DESKTOP_QUERY);
  const touch = useMediaQuery(TOUCH_QUERY);
  const now = useNow(30_000, state.clockOffset);
  const transfers = useStore(uploads.store);
  const onFiles = useCallback((files: File[]) => uploads.add(files), []);
  usePageDropAndPaste(onFiles);

  const preparing = transfers.filter((t) => t.status === 'preparing').length;
  const hasTransfers = transfers.length > 0;
  const empty = tray.files.length === 0;
  const markSeen = (ids?: readonly string[]) => session.markSeen(ids);
  const transferActions = {
    onResume: (id: string) => uploads.resume(id),
    onCancel: (id: string) => uploads.cancel(id),
    onRetry: (id: string) => uploads.retry(id),
    onDismiss: (id: string) => uploads.dismiss(id),
  };
  const header = (
    <AppHeader
      me={tray.me}
      devices={tray.devices}
      code={tray.session.code}
      connection={state.connection}
      layout={desktop ? 'wide' : 'compact'}
      link={hostLink(tray)}
      onRename={rename}
    />
  );
  const invite = <CopyField label="Get another device in" value={tray.session.joinUrl} />;
  const banner = <ConnectionBanner visible={isDown(state.connection)} />;

  useEffect(() => {
    const n = transfers.filter((t) => t.status === 'uploading').length;
    document.title = n > 0 ? `Sending ${plural(n, 'file')} · Consigna` : 'Consigna';
  }, [transfers]);

  if (desktop) {
    return (
      <>
        {banner}
        <DesktopLayout
          header={header}
          left={
            <>
              <DropZone variant="desktop" preparing={preparing} onFiles={onFiles} />
              {hasTransfers && (
                <TransferPanel transfers={transfers} showKeepOpen={false} {...transferActions} />
              )}
              <Spacer />
              {invite}
            </>
          }
          right={
            <>
              <StorageWarning tray={tray} />
              <FileTray
                files={tray.files}
                now={now}
                layout="wide"
                newIds={state.newFileIds}
                onSeen={markSeen}
                onDelete={deleteFiles}
              />
              <Spacer />
              <StorageMeter storage={tray.storage} ttlSeconds={tray.settings.fileTtlSeconds} />
            </>
          }
        />
      </>
    );
  }

  return (
    <>
      {banner}
      <MobileLayout>
        {header}
        {empty && !hasTransfers ? (
          <>
            <Brand subline={`Local session // No internet // ${tray.session.code}`} />
            <DropZone variant="hero" preparing={preparing} onFiles={onFiles} />
          </>
        ) : (
          <DropZone
            variant="compact"
            preparing={preparing}
            label={hasTransfers ? 'Tap to add more' : 'Tap to add files'}
            onFiles={onFiles}
          />
        )}
        <StorageWarning tray={tray} />
        {hasTransfers && <TransferPanel transfers={transfers} showKeepOpen={touch} {...transferActions} />}
        <FileTray
          files={tray.files}
          now={now}
          layout="compact"
          newIds={state.newFileIds}
          onSeen={markSeen}
          onDelete={deleteFiles}
          bareWhenEmpty={!hasTransfers}
        />
        {invite}
        <Spacer />
        <StorageMeter storage={tray.storage} ttlSeconds={tray.settings.fileTtlSeconds} />
      </MobileLayout>
    </>
  );
}

function isDown(c: Connection) {
  return c === 'reconnecting' || c === 'offline';
}
