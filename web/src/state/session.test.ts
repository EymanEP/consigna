import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { file, tray } from '../test/fixtures';
import { SessionController } from './session';

type Listener = (e: MessageEvent<string>) => void;

class FakeEventSource {
  static readonly CLOSED = 2;
  readyState = 1;
  onerror: (() => void) | null = null;
  closed = false;
  private listeners = new Map<string, Listener[]>();
  readonly url: string;
  constructor(url: string) {
    this.url = url;
  }
  addEventListener(type: string, l: Listener) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), l]);
  }
  close() {
    this.closed = true;
    this.readyState = 2;
  }
  emit(type: string, data: unknown) {
    for (const l of this.listeners.get(type) ?? []) l(new MessageEvent(type, { data: JSON.stringify(data) }));
  }
}

function jsonResponse(status: number, body: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }),
  );
}

describe('SessionController', () => {
  let sources: FakeEventSource[];
  let controller: SessionController;

  beforeEach(() => {
    sources = [];
    vi.stubGlobal('EventSource', FakeEventSource);
    controller = new SessionController((url) => {
      const es = new FakeEventSource(url);
      sources.push(es);
      return es as unknown as EventSource;
    });
  });

  afterEach(() => {
    controller.stop();
    vi.unstubAllGlobals();
    window.history.replaceState(null, '', '/');
  });

  it('loads the state, connects, and tracks files from other devices as new', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => jsonResponse(200, tray({ files: [file({ id: 'old' })] }))),
    );
    controller.start();
    await vi.waitFor(() => expect(controller.store.get().phase).toBe('joined'));
    expect(sources).toHaveLength(1);
    expect(controller.store.get().newFileIds.size).toBe(0);

    const es = sources[0];
    if (!es) throw new Error('no event source');
    es.emit(
      'state',
      tray({ files: [file({ id: 'old' }), file({ id: 'theirs' }), file({ id: 'mine', uploaderId: 'd1' })] }),
    );
    expect([...controller.store.get().newFileIds]).toEqual(['theirs']);

    controller.markSeen(['theirs']);
    expect(controller.store.get().newFileIds.size).toBe(0);
  });

  it('shows the join form when not in the session, with the reason from the link', async () => {
    window.history.replaceState(null, '', '/?join=invalid_code');
    vi.stubGlobal(
      'fetch',
      vi.fn(() => jsonResponse(401, { error: { code: 'not_joined', message: 'no' } })),
    );
    controller.start();
    await vi.waitFor(() => expect(controller.store.get().phase).toBe('unjoined'));
    expect(controller.store.get().joinError).toBe('invalid_code');
    expect(window.location.search).toBe('');
  });

  it('ends the session when the device is removed', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => jsonResponse(200, tray())),
    );
    controller.start();
    await vi.waitFor(() => expect(controller.store.get().phase).toBe('joined'));
    sources[0]?.emit('ended', {});
    expect(controller.store.get().phase).toBe('ended');
    expect(controller.store.get().endReason).toBe('removed');
    expect(sources[0]?.closed).toBe(true);

    controller.showJoin();
    expect(controller.store.get().phase).toBe('unjoined');
  });

  it('treats a host restart as a new session', async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn(() => jsonResponse(200, tray()));
    vi.stubGlobal('fetch', fetchMock);
    controller.start();
    await vi.waitFor(() => expect(controller.store.get().phase).toBe('joined'));
    sources[0]?.emit('shutdown', {});
    expect(controller.store.get().endReason).toBe('shutdown');

    fetchMock.mockImplementation(() => jsonResponse(401, { error: { code: 'not_joined', message: 'no' } }));
    await vi.advanceTimersByTimeAsync(2000);
    expect(controller.store.get().phase).toBe('unjoined');
    expect(controller.store.get().joinError).toBe('host_restarted');
    vi.useRealTimers();
  });

  it('marks the connection as reconnecting on stream errors', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => jsonResponse(200, tray())),
    );
    controller.start();
    await vi.waitFor(() => expect(controller.store.get().phase).toBe('joined'));
    const es = sources[0];
    if (!es) throw new Error('no event source');
    es.readyState = 0;
    es.onerror?.();
    expect(controller.store.get().connection).toBe('reconnecting');
    es.emit('state', tray());
    expect(controller.store.get().connection).toBe('live');
  });
});
