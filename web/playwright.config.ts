import { mkdtempSync } from 'node:fs';
import { networkInterfaces, tmpdir } from 'node:os';
import { join } from 'node:path';

import { defineConfig, devices } from '@playwright/test';

// The end-to-end tests drive the real binary (make build). The host's own
// browser reaches it on localhost; "other devices" must come in through a LAN
// address, exactly like a phone would, so the server treats them as guests.
const port = Number(process.env.E2E_PORT ?? 7439);

function lanAddress(): string | undefined {
  for (const list of Object.values(networkInterfaces())) {
    for (const a of list ?? []) {
      if (a.family === 'IPv4' && !a.internal && !a.address.startsWith('169.254.')) return a.address;
    }
  }
  return undefined;
}

const lan = lanAddress();
process.env.E2E_HOST_URL = `http://localhost:${String(port)}`;
process.env.E2E_GUEST_URL = lan ? `http://${lan}:${String(port)}` : '';

const dataDir = mkdtempSync(join(tmpdir(), 'consigna-e2e-'));
const executablePath = process.env.PLAYWRIGHT_CHROMIUM_PATH;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  timeout: 30_000,
  use: {
    trace: 'retain-on-failure',
    reducedMotion: 'reduce',
    ...(executablePath ? { launchOptions: { executablePath } } : {}),
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: `../bin/consigna --port ${String(port)} --no-settings-file --data-dir ${dataDir} --no-qr --log-level warn`,
    url: `http://127.0.0.1:${String(port)}/healthz`,
    reuseExistingServer: false,
    timeout: 20_000,
    stdout: 'ignore',
    stderr: 'pipe',
  },
});
