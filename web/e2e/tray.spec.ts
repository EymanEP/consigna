import { expect, test } from '@playwright/test';

import { deviceName, endSession, guestUrl, hostUrl, inTray, joinAsPhone, openHost } from './helpers';

test.skip(!guestUrl, 'needs a LAN address so a second device can join as a guest');

test.afterEach(async ({ browser }) => {
  // Start every test from a clean session.
  const { ctx, page } = await openHost(browser);
  await endSession(page);
  await ctx.close();
});

test('a phone joins with the link, sends a file, and the host downloads it', async ({ browser }) => {
  const host = await openHost(browser);
  const phone = await joinAsPhone(browser, host.code);
  const phoneName = await deviceName(phone.page);

  // The host sees the phone, with its IP address.
  await host.page.getByRole('tab', { name: /Devices/ }).click();
  await expect(host.page.getByText(phoneName, { exact: true })).toBeVisible();
  await expect(host.page.getByText(new URL(guestUrl).hostname)).toBeVisible();

  // The phone sends a file.
  await expect(phone.page.getByText('Nothing in the tray yet')).toBeVisible();
  await phone.page.locator('input[type=file]').setInputFiles({
    name: 'contract-signed-v2.pdf',
    mimeType: 'application/pdf',
    buffer: Buffer.from('%PDF-1.4 consigna test'),
  });

  // It shows up live on the host's tray, marked as new.
  const tray = await host.ctx.newPage();
  await tray.goto(hostUrl);
  await expect(inTray(tray).getByText('contract-signed-v2.pdf', { exact: true })).toBeVisible();
  await expect(inTray(phone.page).getByText('contract-signed-v2.pdf', { exact: true })).toBeVisible();

  const [download] = await Promise.all([
    tray.waitForEvent('download'),
    tray.getByRole('link', { name: 'Download contract-signed-v2.pdf' }).click(),
  ]);
  expect(download.suggestedFilename()).toBe('contract-signed-v2.pdf');
  const stream = await download.createReadStream();
  const chunks: Buffer[] = [];
  for await (const c of stream) chunks.push(c as Buffer);
  expect(Buffer.concat(chunks).toString()).toBe('%PDF-1.4 consigna test');

  await phone.ctx.close();
  await host.ctx.close();
});

test('new files from other devices are flagged, and deletes sync everywhere', async ({ browser }) => {
  const host = await openHost(browser);
  const phone = await joinAsPhone(browser, host.code);
  const tray = await host.ctx.newPage();
  await tray.goto(hostUrl);
  await expect(tray.getByText('Nothing in the tray yet')).toBeVisible();

  await phone.page.locator('input[type=file]').setInputFiles([
    { name: 'a.txt', mimeType: 'text/plain', buffer: Buffer.from('a') },
    { name: 'b.txt', mimeType: 'text/plain', buffer: Buffer.from('bb') },
  ]);
  await expect(tray.getByRole('button', { name: /2 new files/i })).toBeVisible();
  await tray.getByRole('button', { name: /2 new files/i }).click();
  await expect(tray.getByRole('button', { name: /new file/i })).toHaveCount(0);

  // Delete asks for confirmation, then disappears on the phone too.
  await tray.getByRole('button', { name: 'Delete a.txt' }).click();
  await tray.getByRole('button', { name: 'Confirm: Delete a.txt' }).click();
  await expect(inTray(phone.page).getByText('a.txt', { exact: true })).toHaveCount(0);
  await expect(inTray(phone.page).getByText('b.txt', { exact: true })).toBeVisible();

  await phone.ctx.close();
  await host.ctx.close();
});

test('several files download as one ZIP', async ({ browser }) => {
  const host = await openHost(browser);
  const tray = await host.ctx.newPage();
  await tray.goto(hostUrl);
  await tray.locator('input[type=file]').setInputFiles([
    { name: 'one.txt', mimeType: 'text/plain', buffer: Buffer.from('1') },
    { name: 'two.txt', mimeType: 'text/plain', buffer: Buffer.from('2') },
  ]);
  await expect(inTray(tray).getByText('two.txt', { exact: true })).toBeVisible();
  await expect(inTray(tray).getByText('one.txt', { exact: true })).toBeVisible();

  await tray.getByRole('button', { name: 'Select', exact: true }).click();
  await tray.getByRole('checkbox', { name: 'Select one.txt' }).check();
  await tray.getByRole('checkbox', { name: 'Select two.txt' }).check();
  const [download] = await Promise.all([
    tray.waitForEvent('download'),
    tray.getByRole('button', { name: 'Zip' }).click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/^consigna-.*\.zip$/);
  await host.ctx.close();
});

test('a wrong or old link explains itself and accepts a typed code', async ({ browser }) => {
  const host = await openHost(browser);
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${guestUrl}/t/zzzzzz`);
  await expect(page.getByText('That link has expired')).toBeVisible();
  await page.getByLabel('Session code').fill(host.code);
  await page.getByRole('button', { name: 'Join' }).click();
  await expect(page.getByRole('button', { name: /this device/i })).toBeVisible();
  await ctx.close();
  await host.ctx.close();
});

test('renaming a device shows up on the host', async ({ browser }) => {
  const host = await openHost(browser);
  const phone = await joinAsPhone(browser, host.code);
  await phone.page.getByRole('button', { name: /this device/i }).click();
  const input = phone.page.getByRole('textbox', { name: 'Device name' });
  await input.fill('Ana phone');
  await input.press('Enter');
  await expect(phone.page.getByRole('button', { name: /This device: Ana phone/ })).toBeVisible();
  await host.page.getByRole('tab', { name: /Devices/ }).click();
  await expect(host.page.getByText('Ana phone', { exact: true })).toBeVisible();
  await phone.ctx.close();
  await host.ctx.close();
});

test('host settings reach every device, and ending the session signs them out', async ({ browser }) => {
  const host = await openHost(browser);
  const phone = await joinAsPhone(browser, host.code);
  await expect(phone.page.getByText('Files disappear 24h after they arrive.')).toBeVisible();

  await host.page.getByRole('tab', { name: 'Settings' }).click();
  await host.page.getByLabel('Files disappear after').fill('2');
  await host.page.getByRole('button', { name: 'Save settings' }).click();
  await expect(host.page.getByText('Saved.', { exact: false })).toBeVisible();
  await expect(phone.page.getByText('Files disappear 2h after they arrive.')).toBeVisible();

  // Put it back for the other tests.
  await host.page.getByLabel('Files disappear after').fill('24');
  await host.page.getByRole('button', { name: 'Save settings' }).click();

  await endSession(host.page);
  await expect(phone.page.getByText('You are out of the session')).toBeVisible();
  await phone.page.getByRole('button', { name: 'Join again' }).click();
  await expect(phone.page.getByLabel('Session code')).toBeVisible();
  await phone.ctx.close();
  await host.ctx.close();
});

test('host-only view is refused to other devices', async ({ browser }) => {
  const host = await openHost(browser);
  const phone = await joinAsPhone(browser, host.code);
  await phone.page.goto(`${guestUrl}/host`);
  await expect(phone.page.getByText('Only on the host computer')).toBeVisible();
  await phone.ctx.close();
  await host.ctx.close();
});
