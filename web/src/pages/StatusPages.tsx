import { session, type EndReason } from '../state/session';
import { Button, PrepBars } from '../components/atoms';
import { Brand, Notice } from '../components/molecules';
import { CenteredLayout } from '../components/templates';

/** While the first state loads, or while the host cannot be reached at all. */
export function LoadingPage({ offline }: { offline: boolean }) {
  return (
    <CenteredLayout>
      <Brand subline="Local session // No internet" />
      {offline ? (
        <Notice tone="warning" title="Can't reach the host" live>
          Make sure this device is on the same Wi-Fi as the computer running Consigna, and that Consigna is
          still running. Retrying on its own.
        </Notice>
      ) : (
        <div role="status" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <PrepBars small />
          <span style={{ color: 'var(--c-text-soft)' }}>Connecting to the host…</span>
        </div>
      )}
    </CenteredLayout>
  );
}

/** After the host ended the session, removed this device, or stopped. */
export function EndedPage({ reason }: { reason: EndReason | null }) {
  const stopped = reason === 'shutdown';
  return (
    <CenteredLayout>
      <Brand subline="Local session // No internet" />
      <Notice
        tone="warning"
        title={stopped ? 'The host stopped Consigna' : 'You are out of the session'}
        live
        action={
          !stopped && (
            <Button variant="primary" onClick={() => session.showJoin()}>
              Join again
            </Button>
          )
        }
      >
        {stopped
          ? 'Every file in the tray was deleted. This page reconnects on its own if the host starts it again.'
          : 'The host ended the session or removed this device, and the files you saw are gone. To come back, scan the current QR code or type the new session code.'}
      </Notice>
    </CenteredLayout>
  );
}
