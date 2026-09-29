// Formatting helpers. Byte sizes use decimal units, like the server and like
// macOS, iOS and most file managers.

const UNITS = [
  ['TB', 1e12],
  ['GB', 1e9],
  ['MB', 1e6],
  ['KB', 1e3],
] as const;

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 1000) return `${String(Math.max(0, Math.round(n || 0)))} B`;
  for (const [unit, factor] of UNITS) {
    if (n >= factor) {
      const v = n / factor;
      const text = v >= 100 ? v.toFixed(0) : v.toFixed(1).replace(/\.0$/, '');
      return `${text} ${unit}`;
    }
  }
  return `${String(n)} B`;
}

/** Compact age such as "now", "2m", "3h", "2d". */
export function formatAge(fromIso: string, now: number): string {
  const s = Math.max(0, Math.floor((now - Date.parse(fromIso)) / 1000));
  if (s < 45) return 'now';
  if (s < 3600) return `${String(Math.max(1, Math.round(s / 60)))}m`;
  if (s < 86400) return `${String(Math.floor(s / 3600))}h`;
  return `${String(Math.floor(s / 86400))}d`;
}

/** "23h", "45m", "under a minute" until the given time. */
export function formatRemaining(toIso: string, now: number): string {
  const s = Math.floor((Date.parse(toIso) - now) / 1000);
  if (s < 60) return 'under a minute';
  if (s < 3600) return `${String(Math.floor(s / 60))}m`;
  if (s < 86400) {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    return m > 0 && h < 10 ? `${String(h)}h ${String(m)}m` : `${String(h)}h`;
  }
  return `${String(Math.floor(s / 86400))}d`;
}

/** A duration in seconds as "24h", "7 days", "90 minutes". */
export function formatDuration(seconds: number): string {
  if (seconds % 86400 === 0 && seconds >= 2 * 86400) return `${String(seconds / 86400)} days`;
  if (seconds % 3600 === 0) return `${String(seconds / 3600)}h`;
  if (seconds % 60 === 0) return `${String(seconds / 60)} minutes`;
  return `${String(seconds)}s`;
}

export function formatRate(bytesPerSecond: number): string {
  return `${formatBytes(bytesPerSecond)}/s`;
}

export function formatEta(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '';
  if (seconds < 60) return `${String(Math.max(1, Math.round(seconds)))}s left`;
  if (seconds < 3600) return `${String(Math.round(seconds / 60))}m left`;
  const h = Math.floor(seconds / 3600);
  return `${String(h)}h ${String(Math.round((seconds % 3600) / 60))}m left`;
}

/** Short upper-case type label from the extension, e.g. "ZIP", "MP4". */
export function fileTypeLabel(name: string): string {
  const lower = name.toLowerCase();
  if (lower.endsWith('.tar.gz') || lower.endsWith('.tgz')) return 'TGZ';
  const dot = name.lastIndexOf('.');
  if (dot <= 0 || dot === name.length - 1) return 'FILE';
  const ext = name.slice(dot + 1).toUpperCase();
  return /^[A-Z0-9]{1,4}$/.test(ext) ? ext : 'FILE';
}

/**
 * Shortens long names in the middle so both the start and the extension stay
 * visible: "IMG_20260920_14…_HDR.mov".
 */
export function middleTruncate(name: string, max: number): string {
  const chars = Array.from(name);
  if (chars.length <= max) return name;
  const dot = name.lastIndexOf('.');
  const extLen = dot > 0 && name.length - dot <= 8 ? Array.from(name.slice(dot)).length : 0;
  const tail = Math.min(chars.length - 1, extLen + Math.max(4, Math.floor((max - extLen) / 3)));
  const head = Math.max(1, max - tail - 1);
  return chars.slice(0, head).join('') + '…' + chars.slice(chars.length - tail).join('');
}

export function plural(n: number, one: string, many = one + 's'): string {
  return `${String(n)} ${n === 1 ? one : many}`;
}
