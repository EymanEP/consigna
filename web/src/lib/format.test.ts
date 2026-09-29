import { describe, expect, it } from 'vitest';

import {
  fileTypeLabel,
  formatAge,
  formatBytes,
  formatDuration,
  formatEta,
  formatRemaining,
  middleTruncate,
  plural,
} from './format';

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [999, '999 B'],
    [1000, '1 KB'],
    [310_000, '310 KB'],
    [1_400_000, '1.4 MB'],
    [842_000_000, '842 MB'],
    [1_200_000_000, '1.2 GB'],
    [10e9, '10 GB'],
    [Number.NaN, '0 B'],
  ])('%d -> %s', (n, want) => {
    expect(formatBytes(n)).toBe(want);
  });
});

describe('time formatting', () => {
  const now = Date.parse('2026-09-20T12:00:00Z');
  it('formats ages compactly', () => {
    expect(formatAge('2026-09-20T11:59:50Z', now)).toBe('now');
    expect(formatAge('2026-09-20T11:58:00Z', now)).toBe('2m');
    expect(formatAge('2026-09-20T09:00:00Z', now)).toBe('3h');
    expect(formatAge('2026-09-18T12:00:00Z', now)).toBe('2d');
    expect(formatAge('2026-09-20T12:05:00Z', now)).toBe('now'); // clock skew
  });
  it('formats time remaining', () => {
    expect(formatRemaining('2026-09-20T12:00:30Z', now)).toBe('under a minute');
    expect(formatRemaining('2026-09-20T12:45:00Z', now)).toBe('45m');
    expect(formatRemaining('2026-09-20T14:30:00Z', now)).toBe('2h 30m');
    expect(formatRemaining('2026-09-21T11:00:00Z', now)).toBe('23h');
  });
  it('formats durations and ETAs', () => {
    expect(formatDuration(86400)).toBe('24h');
    expect(formatDuration(7 * 86400)).toBe('7 days');
    expect(formatDuration(90 * 60)).toBe('90 minutes');
    expect(formatEta(14)).toBe('14s left');
    expect(formatEta(600)).toBe('10m left');
    expect(formatEta(Number.POSITIVE_INFINITY)).toBe('');
  });
});

describe('names', () => {
  it('labels file types', () => {
    expect(fileTypeLabel('site-photos.zip')).toBe('ZIP');
    expect(fileTypeLabel('backup.tar.gz')).toBe('TGZ');
    expect(fileTypeLabel('README')).toBe('FILE');
    expect(fileTypeLabel('.env')).toBe('FILE');
    expect(fileTypeLabel('weird.extension-too-long')).toBe('FILE');
  });

  it('truncates in the middle and keeps the extension', () => {
    expect(middleTruncate('short.pdf', 20)).toBe('short.pdf');
    const out = middleTruncate('IMG_20260920_143022_HDR_extra_long_name.mov', 24);
    expect(Array.from(out)).toHaveLength(24);
    expect(out.startsWith('IMG_')).toBe(true);
    expect(out.endsWith('.mov')).toBe(true);
    expect(out).toContain('…');
  });

  it('does not split emoji', () => {
    const out = middleTruncate('📄📄📄📄📄📄📄📄📄📄📄📄📄📄📄📄📄📄📄📄.txt', 12);
    expect(out).not.toMatch(/�/);
    expect(Array.from(out)).toHaveLength(12);
  });

  it('pluralises', () => {
    expect(plural(1, 'file')).toBe('1 file');
    expect(plural(3, 'file')).toBe('3 files');
    expect(plural(2, 'device')).toBe('2 devices');
  });
});
