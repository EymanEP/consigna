# HTTP API (v1)

All endpoints are under `/api/v1`. Responses are JSON unless noted. Errors look like:

```json
{ "error": { "code": "tray_full", "message": "There is not enough space left in the tray for this file." } }
```

Authentication is the `consigna` cookie set when joining. Requests that change something must come from the app's own origin (checked with `Origin` / `Sec-Fetch-Site`). The JSON types are defined in `internal/server/views.go` and mirrored in `web/src/lib/types.ts`.

## Joining

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/t/{code}` | Join link. Sets the cookie and redirects to `/`, or to `/?join=invalid_code`, `rate_limited` or `session_full`. |
| POST | `/api/v1/join` | Body `{"code": "9FQ2XK"}` (case-insensitive). Returns the state. `403 invalid_code`, `429 rate_limited`, `503 session_full`. |
| POST | `/api/v1/leave` | Signs this device out. `204`. |

## State and live updates

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/api/v1/state` | The tray as seen by this device: `me`, `session` (`code`, `joinUrl`), `devices`, `files`, `storage`, `settings`, `serverTime`, and `host` for the host. |
| GET | `/api/v1/events` | Server-Sent Events. `state` carries the same JSON as `/state` after every change; `ended` means this device was removed or the session ended; `shutdown` means the host is stopping. Comment lines keep the connection alive. |
| PATCH | `/api/v1/me` | Body `{"name": "Work laptop"}`. `400 invalid_name`, `409 name_taken`. |

## Files

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/api/v1/files/{id}/content` | Download, always as an attachment. Supports `Range` and `If-Range`. |
| DELETE | `/api/v1/files/{id}` | `204`, or `404 file_not_found`. |
| POST | `/api/v1/files/delete` | Body `{"ids": [...]}` (1–1000). Returns `{"deleted": n}`. |
| GET | `/api/v1/archive?ids=a,b,c` | A ZIP of the files, streamed. `404` if any is gone. |

## Uploads (tus 1.0)

Core protocol with the `creation` and `termination` extensions. `Upload-Length` is required (no deferred length); the file name comes from the `filename` key in `Upload-Metadata`.

| Method | Path | Notes |
| --- | --- | --- |
| OPTIONS | `/api/v1/uploads/` | Capabilities. |
| POST | `/api/v1/uploads/` | Creates an upload and reserves its size. `201` with `Location`; `413 tray_full`. |
| HEAD | `/api/v1/uploads/{id}` | Current `Upload-Offset`. Uploads are private to the device that created them. |
| PATCH | `/api/v1/uploads/{id}` | Appends at `Upload-Offset` (`Content-Type: application/offset+octet-stream`). `409` on a wrong offset, `423` if another request still holds the upload. |
| DELETE | `/api/v1/uploads/{id}` | Cancels and frees the space. |

## Host only

Only for browsers on the host computer (loopback, not relayed by a proxy).

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/api/v1/host/qr.svg?ip=…` | QR code of the join link for one of this machine's addresses. |
| PUT | `/api/v1/host/settings` | Body `{"trayLimit": 10000000000, "fileTtlSeconds": 86400}`. Saved and applied at once. |
| POST | `/api/v1/host/rotate-code` | New join code; devices already in stay in. |
| POST | `/api/v1/host/end-session` | Deletes every file and signs every device out. |
| DELETE | `/api/v1/host/devices/{id}` | Signs a device out and cancels its uploads. |

## Other

`GET /healthz` returns `{"status":"ok"}` without authentication.
