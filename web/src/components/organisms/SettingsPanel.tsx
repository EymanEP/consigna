import { useEffect, useId, useState, type SyntheticEvent } from 'react';

import { ApiError, api } from '../../lib/api';
import { formatBytes } from '../../lib/format';
import type { Settings, Storage } from '../../lib/types';
import { Button, Select, TextInput } from '../atoms';
import { Field, SectionHeader } from '../molecules';
import styles from './HostPanels.module.css';

type TtlUnit = 'minutes' | 'hours' | 'days';
const unitSeconds: Record<TtlUnit, number> = { minutes: 60, hours: 3600, days: 86400 };

function splitTtl(seconds: number): { value: string; unit: TtlUnit } {
  if (seconds % 86400 === 0 && seconds >= 86400 * 2) return { value: String(seconds / 86400), unit: 'days' };
  if (seconds % 3600 === 0) return { value: String(seconds / 3600), unit: 'hours' };
  return { value: String(Math.round(seconds / 60)), unit: 'minutes' };
}

interface SettingsPanelProps {
  settings: Settings;
  storage: Storage;
}

/** Tray size limit and file expiry, saved on the host and applied at once. */
export function SettingsPanel({ settings, storage }: SettingsPanelProps) {
  const initialTtl = splitTtl(settings.fileTtlSeconds);
  const [limitGb, setLimitGb] = useState(String(settings.trayLimit / 1e9));
  const [ttl, setTtl] = useState(initialTtl.value);
  const [unit, setUnit] = useState<TtlUnit>(initialTtl.unit);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const ids = {
    limit: useId(),
    limitHint: useId(),
    ttl: useId(),
    ttlHint: useId(),
    unit: useId(),
    error: useId(),
  };

  useEffect(() => {
    if (!saved) return;
    const t = setTimeout(() => setSaved(false), 2500);
    return () => clearTimeout(t);
  }, [saved]);

  const submit = async (e: SyntheticEvent) => {
    e.preventDefault();
    setError(null);
    const gb = Number(limitGb);
    const amount = Number(ttl);
    if (!Number.isFinite(gb) || gb <= 0) {
      setError('Enter the tray size in GB, for example 10.');
      return;
    }
    if (!Number.isFinite(amount) || amount <= 0) {
      setError('Enter how long files stay, for example 24 hours.');
      return;
    }
    setSaving(true);
    try {
      await api.host.saveSettings({
        trayLimit: Math.round(gb * 1e9),
        fileTtlSeconds: Math.round(amount * unitSeconds[unit]),
      });
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not save the settings.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className={styles.panel} aria-labelledby="settings-title">
      <SectionHeader id="settings-title" title="Settings" lit />
      <form
        className={styles.form}
        onSubmit={(e) => void submit(e)}
        noValidate
        aria-describedby={error ? ids.error : undefined}
      >
        <Field
          htmlFor={ids.limit}
          label="Tray size limit (GB)"
          hint={`Counts every file plus uploads in progress. ${formatBytes(storage.used + storage.reserved)} in use now. Lowering it never deletes files; it only stops new ones until there is room.`}
          hintId={ids.limitHint}
        >
          <TextInput
            id={ids.limit}
            className={styles.number}
            type="number"
            inputMode="decimal"
            min="0.01"
            step="any"
            value={limitGb}
            onChange={(e) => setLimitGb(e.target.value)}
            aria-describedby={ids.limitHint}
            required
          />
        </Field>
        <Field
          htmlFor={ids.ttl}
          label="Files disappear after"
          hint="Applies to files already in the tray too: making it shorter removes older files right away. Between 1 minute and 30 days."
          hintId={ids.ttlHint}
        >
          <span className={styles.inline}>
            <TextInput
              id={ids.ttl}
              className={styles.number}
              type="number"
              inputMode="numeric"
              min="1"
              step="1"
              value={ttl}
              onChange={(e) => setTtl(e.target.value)}
              aria-describedby={ids.ttlHint}
              required
            />
            <label className="visually-hidden" htmlFor={ids.unit}>
              Unit
            </label>
            <Select
              id={ids.unit}
              className={styles.unit}
              value={unit}
              onChange={(e) => setUnit(e.target.value as TtlUnit)}
            >
              <option value="minutes">minutes</option>
              <option value="hours">hours</option>
              <option value="days">days</option>
            </Select>
          </span>
        </Field>
        {error && (
          <p id={ids.error} role="alert" style={{ color: 'var(--c-danger)', fontSize: 'var(--text-small)' }}>
            {error}
          </p>
        )}
        <div className={styles.formActions}>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save settings'}
          </Button>
          <span className={styles.saved} role="status">
            {saved ? 'Saved. Every device sees the change now.' : ''}
          </span>
        </div>
      </form>
    </section>
  );
}
