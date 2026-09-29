import { useId, useState, type SyntheticEvent } from 'react';

import { ApiError } from '../lib/api';
import { session } from '../state/session';
import { Button, TextInput } from '../components/atoms';
import { Brand, Notice } from '../components/molecules';
import { CenteredLayout } from '../components/templates';

const reasons: Record<string, { title: string; body: string }> = {
  invalid_code: {
    title: 'That link has expired',
    body: 'The host may have started a new session or changed the code. Scan the QR code on the host again, or type the code shown there.',
  },
  rate_limited: {
    title: 'Too many wrong codes',
    body: 'Wait a few minutes, then try again with the code shown on the host.',
  },
  host_restarted: {
    title: 'The host restarted Consigna',
    body: 'That started a new session with a new code, and the old files are gone. Scan the new QR code on the host, or type the code it shows.',
  },
  session_full: {
    title: 'The session is full',
    body: 'Ask the host to remove a device they no longer use.',
  },
};

/** For a device that is not in the session: explain why and take a code. */
export function JoinPage({ reason }: { reason: string | null }) {
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const inputId = useId();
  const errorId = useId();
  const known = reason ? reasons[reason] : undefined;

  const submit = async (e: SyntheticEvent) => {
    e.preventDefault();
    if (code.trim().length === 0) {
      setError('Type the 6-character code shown on the host.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await session.join(code);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not join.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <CenteredLayout>
      <Brand subline="Local session // No internet" />
      {known ? (
        <Notice tone="warning" title={known.title} live>
          {known.body}
        </Notice>
      ) : (
        <div>
          <h1 style={{ fontSize: 21, fontWeight: 600 }}>Join the session</h1>
          <p
            style={{
              marginTop: 8,
              color: 'var(--c-text-soft)',
              fontSize: 13,
              lineHeight: 1.6,
              textShadow: 'none',
            }}
          >
            Scan the QR code on the computer running Consigna, or type the session code it shows.
          </p>
        </div>
      )}
      <form
        onSubmit={(e) => void submit(e)}
        noValidate
        style={{ display: 'flex', flexDirection: 'column', gap: 12 }}
      >
        <label htmlFor={inputId} style={{ fontSize: 14, fontWeight: 600 }}>
          Session code
        </label>
        <div style={{ display: 'flex', gap: 8 }}>
          <TextInput
            id={inputId}
            value={code}
            onChange={(e) =>
              setCode(
                e.target.value
                  .toUpperCase()
                  .replace(/[^A-Z0-9]/g, '')
                  .slice(0, 6),
              )
            }
            placeholder="9FQ2XK"
            autoComplete="one-time-code"
            autoCapitalize="characters"
            spellCheck={false}
            inputMode="text"
            aria-invalid={error !== null}
            aria-describedby={error ? errorId : undefined}
            style={{ letterSpacing: '0.3em', fontSize: 20 }}
          />
          <Button type="submit" variant="primary" disabled={busy}>
            {busy ? 'Joining…' : 'Join'}
          </Button>
        </div>
        {error && (
          <p id={errorId} role="alert" style={{ color: 'var(--c-danger)', fontSize: 12 }}>
            {error}
          </p>
        )}
      </form>
    </CenteredLayout>
  );
}
