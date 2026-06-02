# Atmos CLI

Unofficial CLI for controlling the Atmos Agent VPN tunnel.

This project is experimental and is not affiliated with Axis Security or the
Atmos Agent product.

## Requirements

- Atmos Agent installed locally.
- Go 1.26.3 or newer to build from source.

## Install

Install the RPM on Fedora or another RPM-based distro:

```sh
sudo dnf install ./atmosctl-*.rpm
```

Or install from the Linux tarball:

```sh
tar -xzf atmosctl_*_linux_amd64.tar.gz
install -Dm755 atmosctl ~/.local/bin/atmosctl
```

## Build From Source

```sh
make build
install -Dm755 bin/atmosctl ~/.local/bin/atmosctl
```

## Release Packages

Snapshot packages can be built locally:

```sh
make package
```

Tagged releases publish a Linux tarball, RPM, and checksum file from GitHub
Actions. Create and push a semver tag such as `v0.1.0` to publish a release.

## Usage

```sh
atmosctl version
atmosctl vpn status
atmosctl vpn pause
atmosctl vpn resume
atmosctl gui-autostart status
atmosctl gui-autostart enable
atmosctl gui-autostart disable
```

Use `--json` before the command for machine-readable output:

```sh
atmosctl --json version
atmosctl --json vpn status
atmosctl --json gui-autostart status
```

`gui-autostart` controls whether the Atmos GUI opens at login. Disabling it
adds a per-user hidden desktop override for the packaged GUI entry and enables
the user service so the backend can still start at desktop login without opening
the GUI window.
Enabling it removes that override and restores the packaged GUI login behavior.

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
