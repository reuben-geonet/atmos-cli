# Atmos CLI

Unofficial CLI for controlling the Atmos Agent VPN tunnel.

This project is experimental and is not affiliated with Axis Security or the
Atmos Agent product.

## Requirements

- Atmos Agent installed locally.
- Go 1.26 or newer to build from source.

## Build

```sh
make build
install -Dm755 bin/atmosctl ~/.local/bin/atmosctl
```

## Usage

```sh
atmosctl version
atmosctl vpn status
atmosctl vpn pause
atmosctl vpn resume
atmosctl autostart status
atmosctl autostart enable
atmosctl autostart disable
```

Use `--json` before the command for machine-readable output:

```sh
atmosctl --json version
atmosctl --json vpn status
atmosctl --json autostart status
```

`--json vpn status` includes the user service state so integrations can
distinguish a disconnected tunnel from an inactive Atmos backend:

```json
{
  "schemaVersion": 1,
  "state": "disconnected",
  "interface": "atmos",
  "addresses": [],
  "reason": "interface_missing",
  "service": "atmos-agent.service",
  "serviceActive": false,
  "serviceState": "inactive"
}
```
