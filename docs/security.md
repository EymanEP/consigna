# Security model

Consigna is built for a network you trust, such as your home Wi-Fi, and for sharing between your own devices or with people next to you. This page explains what it protects against and what it does not.

## Who can get in

- Joining needs the **session code**, which is in the link and the QR code. It is 6 characters from a 29-character alphabet (about 594 million combinations), generated with a cryptographic random source.
- Wrong codes are **rate-limited**: 10 per address and 200 overall per 10 minutes. A session holds at most 256 devices.
- Each joined device gets a random **256-bit token** in an HttpOnly cookie. The server only keeps a SHA-256 hash of it.
- The **host** (a browser on the computer running Consigna, connecting over loopback) is admitted without a code and is the only one that can see IP addresses, change limits, rotate the code, remove devices or end the session. Requests relayed by a proxy (with forwarding headers) are never treated as the host. If you put Consigna behind a reverse proxy on the same machine, start it with `--trust-loopback=false`.
- Anyone who has the link can join, as with any share link. If a link leaked, **rotate the code** (devices already in stay in) or **end the session**.

## What the server defends against

- **Malicious files.** Downloads are always sent as attachments with `application/octet-stream`, `X-Content-Type-Options: nosniff` and a sandboxing Content Security Policy, so an uploaded HTML or SVG file cannot run inside Consigna's origin.
- **Path tricks.** File names are cleaned and only shown; contents are stored under random IDs.
- **Cross-site requests (CSRF).** Changes are only accepted from Consigna's own origin (`Origin` and `Sec-Fetch-Site` checks), and the cookie is `SameSite=Lax`.
- **DNS rebinding.** Requests must name the server by IP address, `localhost` or an allowed host name (`--allow-host`), so a web page on another domain cannot reach Consigna through your browser.
- **Clickjacking and injection.** A strict Content Security Policy (`default-src 'self'`, no inline scripts), `frame-ancestors 'none'`, `Referrer-Policy: no-referrer`.
- **Filling the disk.** The tray limit counts uploads in progress, and space is reserved before any byte is written.
- **Stalled connections.** Idle transfers are cut after 60 seconds; headers must arrive within 10.

## What it does not do (yet)

- **No encryption on the LAN.** Traffic is plain HTTP. Anyone who can watch your network traffic (for example on an open Wi-Fi) can see files in transit and the session cookie. Use Consigna on networks you trust. Optional HTTPS is on the roadmap.
- **Files at rest are not encrypted** on the host. They live in a directory only your user can read (mode 0700) and are deleted when they expire and when Consigna stops. If Consigna is killed abruptly, the next start removes the leftovers.
- **Everyone in a session is trusted equally.** Any device can download or delete any file.
- **Not ready for the internet** as it is. Do not expose the port through your router. See [self-hosting.md](self-hosting.md) for the planned remote mode.

## Reporting a vulnerability

Please see [SECURITY.md](../SECURITY.md).
