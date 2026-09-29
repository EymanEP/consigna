import { useEffect } from 'react';

import { usePath } from './lib/router';
import { useStore } from './lib/store';
import { session } from './state/session';
import { CrtScreen } from './components/templates';
import { HostPage } from './pages/HostPage';
import { JoinPage } from './pages/JoinPage';
import { EndedPage, LoadingPage } from './pages/StatusPages';
import { TrayPage } from './pages/TrayPage';

function Screen() {
  const state = useStore(session.store);
  const path = usePath();

  useEffect(() => {
    session.start();
    return () => session.stop();
  }, []);

  switch (state.phase) {
    case 'loading':
      return <LoadingPage offline={state.connection === 'offline'} />;
    case 'unjoined':
      return <JoinPage reason={state.joinError} />;
    case 'ended':
      return <EndedPage reason={state.endReason} />;
    case 'joined':
      if (!state.tray) return <LoadingPage offline={false} />;
      return path === '/host' ? (
        <HostPage tray={state.tray} state={state} />
      ) : (
        <TrayPage tray={state.tray} state={state} />
      );
  }
}

export function App() {
  return (
    <CrtScreen>
      <Screen />
    </CrtScreen>
  );
}
