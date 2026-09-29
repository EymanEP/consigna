import * as tus from 'tus-js-client';

import { urls } from '../lib/api';
import { createStore } from '../lib/store';

export type TransferStatus = 'preparing' | 'queued' | 'uploading' | 'paused' | 'failed' | 'done';

export interface Transfer {
  id: string;
  name: string;
  size: number;
  status: TransferStatus;
  /** Bytes the server has confirmed or received so far. */
  sent: number;
  /** Smoothed upload speed in bytes per second. */
  rate: number;
  /** Explanation for paused and failed transfers. */
  message: string | null;
}

/** What to do after an upload error. */
export type ErrorAction = { kind: 'pause' } | { kind: 'fail'; message: string };

/**
 * Decides whether an error is temporary (network drop, server busy) and
 * should pause and retry, or permanent and should fail.
 */
export function classifyError(status: number): ErrorAction {
  switch (true) {
    case status === 0: // no response: network gone, phone asleep
    case status === 404: // upload expired on the server; tus starts a new one
    case status === 408:
    case status === 409: // offset mismatch; tus re-syncs with HEAD
    case status === 423: // another request still holds the upload
    case status === 429:
    case status >= 500:
      return { kind: 'pause' };
    case status === 413:
      return {
        kind: 'fail',
        message: 'Not enough space left in the tray. Delete something, or ask the host to raise the limit.',
      };
    case status === 401:
      return { kind: 'fail', message: 'This device is no longer in the session.' };
    default:
      return { kind: 'fail', message: 'The host refused this file.' };
  }
}

const RETRY_DELAYS = [1000, 2000, 4000, 8000, 15000, 30000];
const CHUNK_SIZE = 64 * 1024 * 1024;
const PROGRESS_INTERVAL = 120;

interface Entry {
  transfer: Transfer;
  file: File;
  upload: tus.Upload | null;
  retryTimer: ReturnType<typeof setTimeout> | undefined;
  attempt: number;
  lastSample: { at: number; bytes: number } | null;
}

let nextId = 0;

/**
 * Sends files one at a time with the tus resumable upload protocol. A dropped
 * connection pauses the transfer and it resumes on its own, from the last byte
 * the server has, when the network returns.
 */
export class UploadManager {
  readonly store = createStore<readonly Transfer[]>([]);
  private readonly entries = new Map<string, Entry>();
  private flushTimer: ReturnType<typeof setTimeout> | undefined;
  private listening = false;

  add(files: readonly File[]): void {
    if (files.length === 0) return;
    this.listen();
    for (const file of files) {
      const id = `t${String(++nextId)}`;
      const entry: Entry = {
        transfer: {
          id,
          name: file.name,
          size: file.size,
          status: 'preparing',
          sent: 0,
          rate: 0,
          message: null,
        },
        file,
        upload: null,
        retryTimer: undefined,
        attempt: 0,
        lastSample: null,
      };
      this.entries.set(id, entry);
      void this.prepare(entry);
    }
    this.flush();
  }

  /** Resumes a paused transfer now instead of waiting for the next retry. */
  resume(id: string): void {
    const e = this.entries.get(id);
    if (e?.transfer.status !== 'paused') return;
    clearTimeout(e.retryTimer);
    e.attempt = 0;
    this.begin(e);
  }

  /** Tries a failed transfer again. */
  retry(id: string): void {
    const e = this.entries.get(id);
    if (e?.transfer.status !== 'failed') return;
    e.attempt = 0;
    this.update(e, { status: 'queued', message: null });
    this.pump();
  }

  /** Stops a transfer and discards what was sent. */
  cancel(id: string): void {
    const e = this.entries.get(id);
    if (!e) return;
    clearTimeout(e.retryTimer);
    this.entries.delete(id);
    if (e.upload && e.transfer.status !== 'done') {
      // Terminates the upload on the server and forgets the resume point.
      e.upload.abort(true).catch(() => undefined);
    }
    this.flush();
    this.pump();
  }

  /** Removes a failed transfer from the list. */
  dismiss(id: string): void {
    const e = this.entries.get(id);
    if (e?.transfer.status !== 'failed') return;
    this.entries.delete(id);
    this.flush();
  }

  /** True while anything is still being sent or waiting to be. */
  busy(): boolean {
    return [...this.entries.values()].some(
      (e) => e.transfer.status !== 'failed' && e.transfer.status !== 'done',
    );
  }

  private async prepare(e: Entry): Promise<void> {
    const upload = new tus.Upload(e.file, {
      endpoint: urls.uploads,
      chunkSize: CHUNK_SIZE,
      retryDelays: null,
      metadata: { filename: e.file.name, filetype: e.file.type || 'application/octet-stream' },
      storeFingerprintForResuming: true,
      removeFingerprintOnSuccess: true,
      onProgress: (sent) => this.onProgress(e, sent),
      onSuccess: () => this.onSuccess(e),
      onError: (err) => this.onError(e, err),
    });
    e.upload = upload;
    try {
      // Pick up where an earlier visit left off (e.g. the page reloaded).
      const previous = await upload.findPreviousUploads();
      const last = previous[0];
      if (last) upload.resumeFromPreviousUpload(last);
    } catch {
      // Resume data is optional.
    }
    if (!this.entries.has(e.transfer.id)) return; // cancelled meanwhile
    this.update(e, { status: 'queued' });
    this.pump();
  }

  private pump(): void {
    const all = [...this.entries.values()];
    if (all.some((e) => e.transfer.status === 'uploading' || e.transfer.status === 'paused')) return;
    const next = all.find((e) => e.transfer.status === 'queued');
    if (next) this.begin(next);
  }

  private begin(e: Entry): void {
    e.lastSample = null;
    this.update(e, { status: 'uploading', message: null, rate: 0 });
    e.upload?.start();
  }

  private onProgress(e: Entry, sent: number): void {
    const now = performance.now();
    let rate = e.transfer.rate;
    if (e.lastSample === null) {
      e.lastSample = { at: now, bytes: sent };
    } else if (now - e.lastSample.at >= 400) {
      const inst = ((sent - e.lastSample.bytes) * 1000) / (now - e.lastSample.at);
      rate = rate === 0 ? inst : rate * 0.7 + inst * 0.3;
      e.lastSample = { at: now, bytes: sent };
    }
    e.transfer = { ...e.transfer, sent, rate: Math.max(0, rate) };
    this.scheduleFlush();
  }

  private onSuccess(e: Entry): void {
    this.update(e, { status: 'done', sent: e.transfer.size, message: null });
    // The file now shows in the shared tray; drop the transfer shortly after.
    setTimeout(() => {
      if (this.entries.get(e.transfer.id)?.transfer.status === 'done') {
        this.entries.delete(e.transfer.id);
        this.flush();
      }
    }, 1200);
    this.pump();
  }

  private onError(e: Entry, err: Error): void {
    if (!this.entries.has(e.transfer.id)) return;
    const status = err instanceof tus.DetailedError ? (err.originalResponse?.getStatus() ?? 0) : 0;
    const action = classifyError(status);
    if (action.kind === 'fail') {
      this.update(e, { status: 'failed', message: action.message, rate: 0 });
      this.pump();
      return;
    }
    this.update(e, { status: 'paused', message: null, rate: 0 });
    const delay = RETRY_DELAYS[Math.min(e.attempt, RETRY_DELAYS.length - 1)] ?? 30000;
    e.attempt++;
    clearTimeout(e.retryTimer);
    e.retryTimer = setTimeout(() => {
      if (e.transfer.status === 'paused') this.begin(e);
    }, delay);
  }

  private update(e: Entry, patch: Partial<Transfer>): void {
    e.transfer = { ...e.transfer, ...patch };
    this.flush();
  }

  private scheduleFlush(): void {
    this.flushTimer ??= setTimeout(() => {
      this.flushTimer = undefined;
      this.flush();
    }, PROGRESS_INTERVAL);
  }

  private flush(): void {
    clearTimeout(this.flushTimer);
    this.flushTimer = undefined;
    this.store.set([...this.entries.values()].map((e) => e.transfer));
  }

  /** Resume paused transfers as soon as the network or the tab comes back. */
  private listen(): void {
    if (this.listening) return;
    this.listening = true;
    const wake = (): void => {
      if (document.visibilityState !== 'visible') return;
      for (const e of this.entries.values()) {
        if (e.transfer.status === 'paused') this.resume(e.transfer.id);
      }
    };
    window.addEventListener('online', wake);
    document.addEventListener('visibilitychange', wake);
    window.addEventListener('beforeunload', (ev) => {
      if (this.busy()) ev.preventDefault();
    });
  }
}

export const uploads = new UploadManager();
