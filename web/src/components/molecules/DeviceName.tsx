import { useEffect, useId, useRef, useState, type SyntheticEvent, type KeyboardEvent } from 'react';

import { ApiError } from '../../lib/api';
import { Icon, IconButton, TextInput } from '../atoms';
import styles from './DeviceName.module.css';

interface DeviceNameProps {
  name: string;
  onRename: (name: string) => Promise<void>;
}

/** This device's name; tap to rename it inline. */
export function DeviceName({ name, onRename }: DeviceNameProps) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(name);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const errorId = useId();

  useEffect(() => {
    if (!editing) return;
    input.current?.focus();
    input.current?.select();
  }, [editing]);

  const open = () => {
    setValue(name);
    setError(null);
    setEditing(true);
  };

  const submit = async (e: SyntheticEvent) => {
    e.preventDefault();
    const next = value.trim();
    if (next === name) {
      setEditing(false);
      return;
    }
    setSaving(true);
    try {
      await onRename(next);
      setEditing(false);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not rename this device.');
    } finally {
      setSaving(false);
    }
  };

  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') setEditing(false);
  };

  if (!editing) {
    return (
      <button
        type="button"
        className={styles.name}
        onClick={open}
        aria-label={`This device: ${name}. Rename`}
      >
        {name}
        <Icon name="pencil" size={12} strokeWidth={2} className={styles.pencil} />
      </button>
    );
  }
  return (
    <form className={styles.form} onSubmit={(e) => void submit(e)} onKeyDown={onKey}>
      <TextInput
        ref={input}
        className={styles.input}
        value={value}
        maxLength={32}
        onChange={(e) => setValue(e.target.value)}
        aria-label="Device name"
        aria-invalid={error !== null}
        aria-describedby={error ? errorId : undefined}
        autoComplete="off"
        spellCheck={false}
        disabled={saving}
      />
      <IconButton icon="check" label="Save name" type="submit" disabled={saving} />
      <IconButton icon="close" label="Cancel" onClick={() => setEditing(false)} />
      {error && (
        <span id={errorId} role="alert" className={styles.error}>
          {error}
        </span>
      )}
    </form>
  );
}
