# Project Notes

This repository contains `atmosctl`, an unofficial CLI for controlling the
Atmos Agent VPN tunnel and user startup behavior.

## Key Files

- `cmd/atmosctl/main.go`: CLI implementation.
- `Makefile`: build, test, version, and cleanup shortcuts.
- `README.md`: public project documentation.

## Commands

```bash
make build
make test
make version
```

GitHub Actions runs the same basic verification on push and pull request:

- `go test ./...`
- `go build -buildvcs=false -o bin/atmosctl ./cmd/atmosctl`
- `bin/atmosctl version`
- `bin/atmosctl --json version`

Pin all GitHub Actions `uses:` references to full commit SHAs. Do not use
floating version tags such as `@v6`.

Manual checks:

```bash
bin/atmosctl version
bin/atmosctl --json version
bin/atmosctl vpn status
bin/atmosctl --json vpn status
bin/atmosctl gui-autostart status
bin/atmosctl --json gui-autostart status
```

## Commit Style

Use Conventional Commits for all commits, for example:

```text
feat: add tunnel status command
fix: handle missing Atmos interface
docs: document autostart behavior
```

## Atmos IPC Findings

Atmos uses a local Go pubsub protocol over TCP with JSON messages terminated by
a NUL byte.

Confirmed useful commands:

- Pause: publish `tunnel.Stop` to `127.0.0.1:6668`
- Resume: publish `tunnel.Start` to `127.0.0.1:6668`

`bin/atmosctl vpn pause` and `bin/atmosctl vpn resume` were verified to toggle
the Atmos tunnel without using the Electron GUI.

The older low-level root-daemon subjects `linux.tunnel.stop` and
`linux.tunnel.start` on `127.0.0.1:6667` only stop/start the tunnel process.
They do not update the user agent's desired state, so Atmos can recover the
tunnel after a short delay.

Status is currently inferred from the `atmos` network interface instead of
using pubsub replies. A connected tunnel has an `atmos` interface with a
`100.65.x.x/32` address.

`access-atlas.dev.geonet.cloud` is not a reliable Atmos pause verification URL
on this machine. During testing it resolved to private `172.30.x.x` addresses,
but the route went via `192.168.66.1` on `enp0s13f0u1u4`, not via the `atmos`
interface.

`online.gns.cri.nz` is a better VPN reachability test. While connected it
resolves to `100.65.0.10` and routes as `100.65.0.10 dev atmos src 100.65.0.1`.

## Autostart Handling

The packaged Atmos login entry is `/etc/xdg/autostart/AtmosAgent.desktop`,
which runs `/usr/bin/atmos`. That wrapper starts `atmos-agent.service` if the
service is not running, but opens the Electron GUI if the service is already
active.

`atmosctl gui-autostart disable` creates a per-user XDG autostart override at
`~/.config/autostart/AtmosAgent.desktop` with `Hidden=true` and enables the user
service `atmos-agent.service` for `graphical-session.target`. This keeps the
backend starting at login without the packaged GUI autostart entry opening the
window.

`atmosctl gui-autostart enable` removes that per-user override and disables the
user service, restoring the packaged login behavior.

## Safety Notes

Do not probe the Electron GUI bridge port directly. Earlier raw TCP probing of
the GUI bridge reset the connection and caused the GUI/user service to restart.
