import type { TrayFile, TrayState } from '../lib/types';

export function file(overrides: Partial<TrayFile> = {}): TrayFile {
  return {
    id: 'f1',
    name: 'contract-signed-v2.pdf',
    size: 1_400_000,
    uploaderId: 'd2',
    uploaderName: 'still-pine',
    addedAt: '2026-09-20T11:54:00Z',
    expiresAt: '2026-09-21T11:54:00Z',
    ...overrides,
  };
}

export function tray(overrides: Partial<TrayState> = {}): TrayState {
  return {
    me: { id: 'd1', name: 'quiet-otter', kind: 'phone', online: true, isHost: false, isMe: true },
    session: { code: '9FQ2XK', joinUrl: 'http://192.168.1.47:7431/t/9fq2xk' },
    devices: [
      { id: 'd1', name: 'quiet-otter', kind: 'phone', online: true, isHost: false, isMe: true },
      { id: 'd2', name: 'still-pine', kind: 'desktop', online: true, isHost: true, isMe: false },
    ],
    files: [],
    storage: { used: 0, reserved: 0, limit: 10e9 },
    settings: { trayLimit: 10e9, fileTtlSeconds: 86400 },
    serverTime: new Date().toISOString(),
    ...overrides,
  };
}
