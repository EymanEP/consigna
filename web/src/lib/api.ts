import type { ApiErrorBody, Device, Settings, TrayState } from './types';

/** An error response from the server, or a network failure (status 0). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

function isErrorBody(v: unknown): v is ApiErrorBody {
  if (typeof v !== 'object' || v === null || !('error' in v)) return false;
  const e = v.error;
  return typeof e === 'object' && e !== null && 'code' in e && 'message' in e;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, 'network', 'Cannot reach the host. Check that you are on the same Wi-Fi.');
  }
  if (res.status === 204) return undefined as T;
  const data: unknown = await res.json().catch(() => undefined);
  if (!res.ok) {
    if (isErrorBody(data)) throw new ApiError(res.status, data.error.code, data.error.message);
    throw new ApiError(res.status, 'http_' + String(res.status), 'The host answered with an error.');
  }
  return data as T;
}

export const api = {
  state: () => request<TrayState>('GET', '/api/v1/state'),
  join: (code: string) => request<TrayState>('POST', '/api/v1/join', { code }),
  rename: (name: string) => request<Device>('PATCH', '/api/v1/me', { name }),
  leave: () => request<undefined>('POST', '/api/v1/leave'),
  deleteFile: (id: string) => request<undefined>('DELETE', `/api/v1/files/${encodeURIComponent(id)}`),
  deleteFiles: (ids: string[]) => request<{ deleted: number }>('POST', '/api/v1/files/delete', { ids }),
  host: {
    saveSettings: (s: Settings) => request<Settings>('PUT', '/api/v1/host/settings', s),
    rotateCode: () => request<{ code: string }>('POST', '/api/v1/host/rotate-code'),
    endSession: () => request<{ code: string }>('POST', '/api/v1/host/end-session'),
    removeDevice: (id: string) =>
      request<undefined>('DELETE', `/api/v1/host/devices/${encodeURIComponent(id)}`),
  },
};

export const urls = {
  uploads: '/api/v1/uploads/',
  events: '/api/v1/events',
  file: (id: string) => `/api/v1/files/${encodeURIComponent(id)}/content`,
  archive: (ids: string[]) => `/api/v1/archive?ids=${ids.map(encodeURIComponent).join(',')}`,
  qr: (ip: string) => `/api/v1/host/qr.svg?ip=${encodeURIComponent(ip)}`,
};
