# Atmos CLI

Unofficial CLI for controlling the Atmos Agent VPN tunnel.

This project is experimental and is not affiliated with Axis Security or the
Atmos Agent product.

## Requirements

- Atmos Agent installed locally.
- Go 1.26 or newer to build from source.

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
