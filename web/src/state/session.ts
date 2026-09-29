import { ApiError, api, urls } from '../lib/api';
import { createStore } from '../lib/store';
import type { TrayState } from '../lib/types';

export type Phase = 'loading' | 'joined' | 'unjoined' | 'ended';
export type Connection = 'connecting' | 'live' | 'reconnecting' | 'offline';
export type EndReason = 'removed' | 'shutdown';

export interface SessionState {
  phase: Phase;
  tray: TrayState | null;
  connection: Connection;
  /** Why the last join attempt failed (server error code), if it did. */
  joinError: string | null;
  endReason: EndReason | null;
  /** serverTime minus local time, to render ages with the host's clock. */
  clockOffset: number;
  /** Files other devices added since this page loaded, not yet looked at. */
  newFileIds: ReadonlySet<string>;
}

const initialState: SessionState = {
  phase: 'loading',
  tray: null,
  connection: 'connecting',
  joinError: null,
  endReason: null,
  clockOffset: 0,
  newFileIds: new Set(),
};

type EventSourceFactory = (url: string) => EventSource;

/**
 * Keeps the tray state in sync with the server: an initial fetch, then a
 * Server-Sent Events stream that pushes the full state after every change.
 */
export class SessionController {
  readonly store = createStore<SessionState>(initialState);
  private es: EventSource | null = null;
  private retryTimer: ReturnType<typeof setTimeout> | undefined;
  private retryAttempt = 0;
  private started = false;
  private readonly makeEventSource: EventSourceFactory;

  constructor(makeEventSource: EventSourceFactory = (url) => new EventSource(url)) {
    this.makeEventSource = makeEventSource;
  }

  /** Reads the ?join= hint left by a join link, then loads the state. */
  start(): void {
    if (this.started) return;
    this.started = true;
    const params = new URLSearchParams(window.location.search);
    const joinError = params.get('join');
    if (joinError !== null) {
      params.delete('join');
      const query = params.toString();
      window.history.replaceState(null, '', window.location.pathname + (query ? `?${query}` : ''));
      this.patch({ joinError });
    }
    document.addEventListener('visibilitychange', this.onVisible);
    window.addEventListener('online', this.onVisible);
    void this.load();
  }

  stop(): void {
    this.started = false;
    this.disconnect();
    clearTimeout(this.retryTimer);
    document.removeEventListener('visibilitychange', this.onVisible);
    window.removeEventListener('online', this.onVisible);
  }

  async join(code: string): Promise<void> {
    const tray = await api.join(code);
    this.patch({ joinError: null, endReason: null });
    this.applyState(tray, true);
    this.connect();
  }

  async leave(): Promise<void> {
    await api.leave();
    this.disconnect();
    this.store.set({ ...initialState, phase: 'unjoined', connection: 'offline' });
  }

  /** Marks new files as seen (all of them when ids is omitted). */
  markSeen(ids?: readonly string[]): void {
    const current = this.store.get().newFileIds;
    if (current.size === 0) return;
    if (!ids) {
      this.patch({ newFileIds: new Set() });
      return;
    }
    const next = new Set(current);
    for (const id of ids) next.delete(id);
    if (next.size !== current.size) this.patch({ newFileIds: next });
  }

  /** Leaves the "ended" screen for the join form. */
  showJoin(): void {
    this.disconnect();
    clearTimeout(this.retryTimer);
    this.patch({ phase: 'unjoined', endReason: null, joinError: null, tray: null, newFileIds: new Set() });
  }

  /** Re-fetches the state, e.g. after renaming. */
  refresh(): void {
    void this.load();
  }

  private async load(): Promise<void> {
    clearTimeout(this.retryTimer);
    try {
      const tray = await api.state();
      this.applyState(tray, this.store.get().tray === null);
      this.connect();
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        this.disconnect();
        const { phase, endReason } = this.store.get();
        if (phase === 'ended' && endReason === 'shutdown') {
          // The host is back, but a restart is a new session.
          this.patch({
            phase: 'unjoined',
            joinError: 'host_restarted',
            endReason: null,
            tray: null,
            connection: 'offline',
          });
        } else if (phase === 'joined') {
          this.patch({ phase: 'ended', endReason: 'removed', tray: null, connection: 'offline' });
        } else {
          this.patch({ phase: 'unjoined', tray: null, connection: 'offline' });
        }
        return;
      }
      // Host unreachable: keep trying.
      this.patch({ connection: this.store.get().tray ? 'reconnecting' : 'offline' });
      this.scheduleRetry(() => void this.load());
    }
  }

  private connect(): void {
    if (this.es) return;
    this.patch({ connection: this.store.get().connection === 'live' ? 'live' : 'connecting' });
    const es = this.makeEventSource(urls.events);
    this.es = es;
    es.addEventListener('state', (e) => {
      this.retryAttempt = 0;
      try {
        this.applyState(JSON.parse((e as MessageEvent<string>).data) as TrayState, false);
      } catch {
        // A malformed event is dropped; the next one carries the full state.
      }
    });
    es.addEventListener('ended', () => {
      this.disconnect();
      if (this.store.get().tray?.me.isHost) {
        // The host's own browser is admitted again automatically.
        void this.load();
        return;
      }
      this.patch({ phase: 'ended', endReason: 'removed', connection: 'offline' });
    });
    es.addEventListener('shutdown', () => {
      this.disconnect();
      this.patch({ phase: 'ended', endReason: 'shutdown', connection: 'offline' });
      this.scheduleRetry(() => void this.load());
    });
    es.onerror = () => {
      if (es.readyState === EventSource.CLOSED) {
        // The browser gave up (e.g. a 401): find out why before retrying.
        this.disconnect();
        this.patch({ connection: 'reconnecting' });
        this.scheduleRetry(() => void this.load());
      } else {
        this.patch({ connection: 'reconnecting' });
      }
    };
  }

  private disconnect(): void {
    if (this.es) {
      this.es.close();
      this.es = null;
    }
  }

  private scheduleRetry(fn: () => void): void {
    clearTimeout(this.retryTimer);
    const delays = [1000, 2000, 4000, 8000, 15000];
    const delay = delays[Math.min(this.retryAttempt, delays.length - 1)] ?? 15000;
    this.retryAttempt++;
    this.retryTimer = setTimeout(fn, delay);
  }

  private readonly onVisible = (): void => {
    if (document.visibilityState !== 'visible') return;
    const { phase } = this.store.get();
    if (
      (phase === 'joined' && this.es === null) ||
      (phase === 'ended' && this.store.get().endReason === 'shutdown')
    ) {
      this.retryAttempt = 0;
      void this.load();
    }
  };

  private applyState(tray: TrayState, initial: boolean): void {
    const prev = this.store.get();
    const prevIds = new Set(prev.tray?.files.map((f) => f.id) ?? []);
    const present = new Set(tray.files.map((f) => f.id));
    const newFileIds = new Set([...prev.newFileIds].filter((id) => present.has(id)));
    if (!initial && prev.tray) {
      for (const f of tray.files) {
        if (!prevIds.has(f.id) && f.uploaderId !== tray.me.id) newFileIds.add(f.id);
      }
    }
    this.store.set({
      ...prev,
      phase: 'joined',
      tray,
      connection: 'live',
      endReason: null,
      clockOffset: Date.parse(tray.serverTime) - Date.now(),
      newFileIds,
    });
  }

  private patch(p: Partial<SessionState>): void {
    this.store.set((s) => ({ ...s, ...p }));
  }
}

export const session = new SessionController();
