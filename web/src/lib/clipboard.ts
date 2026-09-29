/**
 * Copies text. The async Clipboard API only exists on secure (HTTPS or
 * localhost) pages, and Consigna is usually opened over plain HTTP on the
 * LAN, so fall back to a hidden textarea and execCommand.
 */
export async function copyText(text: string): Promise<boolean> {
  if (window.isSecureContext && 'clipboard' in navigator) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // fall through to the legacy path
    }
  }
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  ta.style.position = 'fixed';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  try {
    // Deprecated, but the only option on insecure (plain HTTP) origins.
    // eslint-disable-next-line @typescript-eslint/no-deprecated
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    ta.remove();
  }
}
