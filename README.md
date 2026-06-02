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

## Related Projects

- `atmos-gnome`: GNOME Shell Quick Settings integration that uses `atmosctl`.
