// The JSON contract with the Go server. Keep in sync with
// internal/server/views.go.

export type DeviceKind = 'phone' | 'tablet' | 'desktop' | 'unknown';

export interface Device {
  id: string;
  name: string;
  kind: DeviceKind;
  online: boolean;
  isHost: boolean;
  isMe: boolean;
  /** Host view only. */
  ip?: string;
  userAgent?: string;
  joinedAt?: string;
  lastSeen?: string;
}

export interface TrayFile {
  id: string;
  name: string;
  size: number;
  uploaderId: string;
  uploaderName: string;
  addedAt: string;
  expiresAt: string;
}

export interface Storage {
  used: number;
  reserved: number;
  limit: number;
}

export interface Settings {
  trayLimit: number;
  fileTtlSeconds: number;
}

export type AddressKind = 'wifi' | 'ethernet' | 'vpn' | 'virtual' | 'other';

export interface Address {
  interface: string;
  ip: string;
  kind: AddressKind;
  url: string;
}

export interface HostUpload {
  id: string;
  name: string;
  size: number;
  offset: number;
  ownerId: string;
  updatedAt: string;
}

export interface HostInfo {
  version: string;
  addresses: Address[];
  uploads: HostUpload[];
}

export interface TrayState {
  me: Device;
  session: { code: string; joinUrl: string };
  devices: Device[];
  files: TrayFile[];
  storage: Storage;
  settings: Settings;
  serverTime: string;
  host?: HostInfo;
}

export interface ApiErrorBody {
  error: { code: string; message: string };
}
