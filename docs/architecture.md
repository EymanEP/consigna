# Architecture

Consigna is one Go binary that serves a React single-page app and a JSON API. There is no database: the tray is temporary, so its metadata lives in memory and its contents in a private directory that is deleted on shutdown.

```text
cmd/consigna        flags and env, settings, logger → app.Run
internal/app        wiring, listener, graceful shutdown, run directory, banner
internal/server     HTTP: routes, middleware, auth, JSON API, tus, SSE, SPA
internal/store      the tray: files on disk, uploads in progress, quota, expiry
internal/session    join code, devices, tokens, presence, rate limiting
internal/events     coalescing "something changed" fan-out
internal/settings   host-adjustable limits, persisted as JSON
internal/netinfo    LAN addresses, best first
internal/qr         QR codes as SVG (web) and text (terminal)
internal/names      random device names ("quiet-otter")
internal/sizes      decimal byte sizes
web/                React + TypeScript UI, built into web/dist/app and embedded
```

## Request flow

1. A device opens `/t/{code}`. The server checks the code, creates a device with a random 256-bit token (stored hashed), sets it as an HttpOnly cookie and redirects to `/`. The host's own browser, connecting over loopback, is admitted without a code.
2. The app loads `GET /api/v1/state`, then opens `GET /api/v1/events`, a Server-Sent Events stream.
3. Uploads use the tus 1.0 protocol under `/api/v1/uploads/`. Downloads are plain links to `/api/v1/files/{id}/content`, so the browser's own download manager handles them.
4. Every change (file added or deleted, device joined, renamed or left, settings changed) wakes the event hub; each open stream then sends the full current state.

## Design decisions

**Full state on every change, not deltas.** The state is small (a list of files and devices). Sending all of it means a client that slept, lost Wi-Fi or missed events is always correct after the next message, with no replay logic. The hub only carries wake-ups and a pending wake-up absorbs further ones, so publishers never block, slow clients are never dropped, and bursts collapse into one update.

**Our own tus server.** The official `tusd` library brings in cloud-storage SDKs we do not need. The subset Consigna uses (core, creation, termination) is small; the implementation is in `internal/server/uploads.go` and `internal/store`, with tests covering offsets, resumption, takeover and termination.

**Quota by reservation.** Creating an upload reserves its full declared size, so two devices uploading at once cannot overshoot the tray limit. Reservations are released when an upload finishes, is cancelled or goes stale.

**Resumption and takeover.** Bytes received before a connection dies are kept. When a phone reconnects while the server still holds the old, half-dead request, the new request interrupts the old one (by expiring its read deadline) and takes over. Each read also pushes an idle deadline forward, so a stalled connection is cut after 60 seconds, while a healthy transfer can take as long as it needs.

**Idempotent completion.** A finished upload is remembered for a while. A client that never saw the final response asks again and learns the upload is complete, instead of starting over and creating a duplicate.

**Delete while downloading.** Files are reference-counted. Deleting a file hides it at once, and its contents are removed when the last download closes. This also keeps Windows happy, where open files cannot be deleted.

**Names are display only.** Client-supplied file names are cleaned (path parts, control and invisible characters, reserved Windows names, length) and never used as paths. Contents are stored under random IDs.

**No timeouts on whole requests.** Uploads and downloads of large files may take hours, so the HTTP server only bounds header reading; transfers use the per-read and per-write idle deadlines above.

**A run directory with a heartbeat.** Each run writes into its own directory under the per-user cache folder and touches a heartbeat file every minute. On start, directories of runs whose heartbeat is stale (a crash) are removed.

## The web UI

The UI follows atomic design:

```text
src/components/atoms       Button, Icon, Mono, ProgressBar, StatusDot, TextInput, …
src/components/molecules   FileRow, TransferCard, DeviceName, CopyField, Tabs, …
src/components/organisms   AppHeader, DropZone, FileTray, TransferPanel, InvitePanel, …
src/components/templates   CrtScreen, MobileLayout, DesktopLayout, WideLayout
src/pages                  TrayPage, HostPage, JoinPage, status pages
src/state                  session (API + live stream), uploads (tus queue), fx
src/lib                    API client, types, formatting, store, router
src/styles                 design tokens and global styles
```

State lives outside React in small observable stores (`src/lib/store.ts`), so the live connection and the upload queue are plain TypeScript classes that are easy to test; components subscribe with `useSyncExternalStore`.

The upload queue sends one file at a time. Errors are sorted into temporary ones (no response, 5xx, 409, 423, 404), which pause the transfer and retry with backoff and whenever the network or the tab comes back, and permanent ones (a full tray, being signed out), which fail with an explanation. Resume points survive a page reload.

The CRT look (phosphor bloom, drifting band, scanlines, vignette, flicker and glitch bursts) is built with CSS and the Web Animations API. Everything that moves stops in calm mode (a toggle in the header) and for people who prefer reduced motion. Because the glitch briefly transforms the content, anything fixed to the viewport is rendered in a portal.
