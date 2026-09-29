# Running Consigna elsewhere

## On your LAN (supported)

Run the binary on any computer on the network. A few tips:

- **Firewall.** Allow Consigna on private networks when your OS asks. On Linux with `ufw`: `sudo ufw allow 7431/tcp`.
- **Several networks** (Wi-Fi plus Ethernet, a VPN, Docker): the terminal and the host view list every address; pick the one your other device is on.
- **Devices cannot connect?** Guest networks and many office or hotel networks isolate devices from each other (AP or client isolation). Use your main network, or turn on your phone's hotspot and connect the computer to it.
- **A stable name.** If your network resolves `.local` names, start with `--allow-host yourcomputer.local` and use `http://yourcomputer.local:7431`.

### Docker on a LAN

The image is built from the repository's `Dockerfile` (it has not yet been tested in CI):

```sh
docker build -t consigna .
docker run --rm --network host -v consigna-data:/data consigna
```

Use `--network host` (Linux) so Consigna sees the machine's real addresses. With port mapping instead, it shows container addresses and the host view is not available, because requests no longer come from loopback.

## On a VPS or a home server reachable from the internet (planned)

This is not safe yet with the current release; the design for it:

1. **TLS everywhere.** Terminate HTTPS in Consigna itself (automatic Let's Encrypt certificates) or in a reverse proxy such as Caddy. With a proxy, run Consigna with `--trust-loopback=false --bind 127.0.0.1` so proxied requests are never treated as the host.
2. **Strong access.** Replace the 6-character code with long random tokens (128 bits or more) in the link, and optionally let the host approve each new device before it gets in.
3. **A remote host view,** protected by its own secret, since the host is no longer on loopback.
4. **End-to-end encryption.** Encrypt files in the browser before upload, with the key in the link's `#fragment` (which browsers never send to the server). The server would then store only data it cannot read. The store's interface already keeps contents opaque, so this can be added without changing it.
5. **Abuse limits.** Per-device upload rates, smaller default trays, and short expiries.
