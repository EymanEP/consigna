# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Go server sharing a temporary tray over the LAN: join links and codes, device tokens, rate limiting.
- Resumable uploads (tus 1.0) with quota reservations, takeover of stalled connections and idempotent completion.
- Downloads with range support, and multi-file downloads as a streamed ZIP.
- Live updates over Server-Sent Events.
- Host view: QR code with network picker, device list with IP addresses, tray limit and expiry settings, code rotation, end session.
- React web UI in the CRT style, for phones and desktops: drag and drop, paste, multi-select, new-file markers, reconnecting and ended states, calm mode.
- File expiry (24 hours by default), a tray limit (10 GB by default), cleanup on shutdown and after crashes.
- CI, end-to-end tests, release builds for Linux, macOS and Windows, Docker image.
