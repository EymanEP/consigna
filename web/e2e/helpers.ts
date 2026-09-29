import { expect, type Browser, type BrowserContext, type Page } from '@playwright/test';

export const hostUrl = process.env.E2E_HOST_URL ?? '';
export const guestUrl = process.env.E2E_GUEST_URL ?? '';

export const phone = {
  viewport: { width: 390, height: 844 },
  isMobile: true,
  hasTouch: true,
  userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148',
};

/** Opens the host view on "this computer" and returns the current session code. */
export async function openHost(browser: Browser): Promise<{ ctx: BrowserContext; page: Page; code: string }> {
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 860 } });
  const page = await ctx.newPage();
  await page.goto(`${hostUrl}/host`);
  await expect(page.getByRole('heading', { name: 'HOST VIEW' })).toBeVisible();
  const code =
    (
      await page
        .getByText(/^[2-9A-Z]{6}$/)
        .first()
        .textContent()
    )?.trim() ?? '';
  expect(code).toMatch(/^[2-9A-Z]{6}$/);
  return { ctx, page, code };
}

/** A phone joining through the LAN address with the join link. */
export async function joinAsPhone(
  browser: Browser,
  code: string,
): Promise<{ ctx: BrowserContext; page: Page }> {
  const ctx = await browser.newContext(phone);
  const page = await ctx.newPage();
  await page.goto(`${guestUrl}/t/${code.toLowerCase()}`);
  await expect(page.getByRole('button', { name: /this device/i })).toBeVisible();
  return { ctx, page };
}

export async function deviceName(page: Page): Promise<string> {
  const label = await page.getByRole('button', { name: /this device/i }).getAttribute('aria-label');
  return /This device: (.+)\. Rename/.exec(label ?? '')?.[1] ?? '';
}

export async function endSession(page: Page): Promise<void> {
  await page.getByRole('tab', { name: 'Settings' }).click();
  await page.getByRole('button', { name: 'End session' }).click();
  await page.getByRole('button', { name: 'End and delete all' }).click();
}

/** The list of files in the shared tray (not the transfer cards). */
export function inTray(page: Page) {
  return page.getByRole('list', { name: 'Files in the tray' });
}
