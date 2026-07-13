# vpn-bypass

`vpn-bypass` routes selected domains, IPv4 addresses, and IPv4 networks through the normal system gateway while other traffic continues through a full-tunnel VPN.

Supported platforms:

- macOS on Apple silicon and Intel
- Windows x86-64

This is IP-based routing, not true domain-based routing. Domains may use CDNs and change addresses, so the installed background service refreshes routes every 60 seconds. Linux is not supported.

## Install from a release archive

Release archives contain a standalone executable. Go and Python are not required.

### macOS

For Apple silicon:

```bash
curl -LO https://github.com/dimatitov/vpn-bypass/releases/download/v0.1.0/vpn-bypass_Darwin_arm64.tar.gz
tar -xzf vpn-bypass_Darwin_arm64.tar.gz
sudo ./vpn-bypass install
```

Intel Macs use `vpn-bypass_Darwin_x86_64.tar.gz` instead. Installation copies the executable to `/usr/local/bin/vpn-bypass`, creates the default configuration when one does not already exist, and starts the LaunchDaemon.

### Windows

Open PowerShell as Administrator:

```powershell
Invoke-WebRequest `
  https://github.com/dimatitov/vpn-bypass/releases/download/v0.1.0/vpn-bypass_Windows_x86_64.zip `
  -OutFile vpn-bypass.zip
Expand-Archive .\vpn-bypass.zip -DestinationPath .\vpn-bypass-release
.\vpn-bypass-release\vpn-bypass.exe install
```

Installation copies the executable to `C:\Program Files\vpn-bypass\vpn-bypass.exe`, creates the default configuration when needed, and starts a SYSTEM Task Scheduler task.

The installer deliberately does not modify system `PATH`. Use the extracted executable or the installed path:

```powershell
& "C:\Program Files\vpn-bypass\vpn-bypass.exe" status
```

## Build from source

Go 1.24 or newer is required only for source builds:

```bash
git clone https://github.com/dimatitov/vpn-bypass.git
cd vpn-bypass
go build -o vpn-bypass ./cmd/vpn-bypass
sudo ./vpn-bypass install
```

On Windows, build `vpn-bypass.exe` and run `install` from an elevated PowerShell terminal.

## First-time setup

A fresh installation creates an editable JSON configuration with bypass domains for Ozon, Yandex, Avito, and Gosuslugi. Existing configuration is never overwritten during install or upgrade.

Configuration paths:

- macOS: `/Library/Application Support/vpn-bypass/config.json`
- Windows: `C:\ProgramData\vpn-bypass\config.json`

Format:

```json
{
  "domains": ["example.com"],
  "cidrs": ["192.0.2.10/32", "198.51.100.0/24"]
}
```

Use `add`, `remove`, and `list` to edit it. Because configuration is machine-wide, modifications may require an elevated terminal.

## Service and reboot behavior

Both `vpn-bypass install` and the backward-compatible `vpn-bypass service install` use the same installer. The service starts immediately and runs after every reboot:

```text
vpn-bypass watch --interval 60s
```

macOS uses LaunchDaemon `io.github.dimatitov.vpn-bypass`. Windows uses a Task Scheduler task with the same name, running as SYSTEM with highest privileges.

Check the service alone with:

```text
vpn-bypass service status
```

Its stable output is `status: not-installed`, `status: running`, or `status: stopped`.

## Command reference

```text
vpn-bypass add <domain|cidr>
vpn-bypass remove <domain|cidr>
vpn-bypass list
vpn-bypass sync
vpn-bypass clear
vpn-bypass status
vpn-bypass doctor
vpn-bypass watch --interval 60s
vpn-bypass logs
vpn-bypass logs --follow
vpn-bypass version
vpn-bypass install
vpn-bypass uninstall
vpn-bypass uninstall --purge
vpn-bypass service install
vpn-bypass service uninstall
vpn-bypass service status
```

`sync`, `clear`, `install`, and `uninstall` require administrator privileges. `service uninstall` remains an alias for non-purge uninstall; purge is available only as `vpn-bypass uninstall --purge`.

## Status and diagnostics

`vpn-bypass status` reports stable key/value fields:

```text
version: v0.1.0
service: running
direct_gateway: 192.0.2.1
direct_interface: en0
managed_routes: 12
last_successful_sync: 2026-07-13T12:00:00Z
config_path: /Library/Application Support/vpn-bypass/config.json
state_path: /Library/Application Support/vpn-bypass/state.json
```

Run `vpn-bypass doctor` from an elevated terminal. It exits non-zero if an essential check fails and verifies:

- administrator privileges;
- configuration readability;
- DNS resolution;
- direct gateway detection;
- an active VPN route for an unrelated public IP;
- direct routing for at least one configured bypass domain.

## Logs

Use `vpn-bypass logs` or `vpn-bypass logs --follow`.

Stable directories:

- macOS: `/Library/Logs/vpn-bypass/`
- Windows: `C:\ProgramData\vpn-bypass\logs\`

The watcher writes `vpn-bypass.log`. The macOS LaunchDaemon also captures process output in `stdout.log` and `stderr.log`.

## Uninstall and purge

Normal uninstall stops and unregisters the service, clears only routes recorded as owned by vpn-bypass, removes the installed executable, and preserves configuration and logs:

```bash
# macOS
sudo vpn-bypass uninstall

# Windows, in an Administrator PowerShell
vpn-bypass uninstall
```

Remove configuration, state, and logs as well:

```bash
# macOS
sudo vpn-bypass uninstall --purge

# Windows, in an Administrator PowerShell
vpn-bypass uninstall --purge
```

Purge is skipped if owned routes cannot be cleared, preventing loss of the state needed to retry cleanup. On Windows, deleting the currently running installed executable may be scheduled for the next reboot.

## Homebrew readiness

Release archives and checksums are prepared for [dimatitov/homebrew-tap](https://github.com/dimatitov/homebrew-tap). The following commands are intentionally **unavailable until the formula is updated after the first release**:

```bash
brew tap dimatitov/tap
brew install vpn-bypass
```

This repository does not modify the tap formula.

## Troubleshooting

- Run `vpn-bypass status` to verify service and routing state.
- Run elevated `vpn-bypass doctor` for end-to-end diagnostics.
- Inspect `vpn-bypass logs --follow` while reconnecting the VPN.
- Validate `config.json` if configuration loading fails.
- DNS failures or CDN changes may temporarily prevent some routes from being installed; the watcher retries failed additions.
- Security software may block Task Scheduler, PowerShell networking cmdlets, or route changes.

The implementation only manages routes recorded in its state file. It does not inspect VPN credentials, modify VPN configuration, install drivers, open network listeners, or alter Windows `PATH`.

## Manual validation

See [the macOS and Windows smoke-test checklist](docs/manual-smoke-test.md).
