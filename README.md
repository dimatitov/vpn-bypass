# vpn-bypass

`vpn-bypass` routes selected domains, IPv4 addresses, and IPv4 networks through the normal system gateway instead of a full-tunnel VPN. It supports macOS and Windows.

This is IP routing, not true domain-based routing. Domains may resolve to changing CDN addresses, so the background watcher refreshes routes every 60 seconds.

## Installation

Download the archive for your operating system from the GitHub Releases page, extract it, and run the binary from a terminal. You can also build it locally with Go 1.24 or newer:

```text
go build -o vpn-bypass ./cmd/vpn-bypass
```

On Windows, use `vpn-bypass.exe` instead. Verify the build information with:

```text
vpn-bypass version
```

The service installer accepts a local build, an executable extracted from a release archive, or a Homebrew-managed executable. Homebrew symlinks are resolved before the service copy is installed, so later Cellar cleanup does not break the running service.

## Configuration

The machine-wide configuration file is stored at:

- macOS: `/Library/Application Support/vpn-bypass/config.json`
- Windows: `C:\ProgramData\vpn-bypass\config.json`

Its existing JSON format remains stable:

```json
{
  "domains": ["example.com"],
  "cidrs": ["192.0.2.10/32", "198.51.100.0/24"]
}
```

Manage entries with the CLI:

```text
vpn-bypass add example.com
vpn-bypass add 198.51.100.0/24
vpn-bypass remove example.com
vpn-bypass list
```

The configuration is machine-wide, so modifying it may require an elevated terminal. Routes can also be refreshed or removed manually with `vpn-bypass sync` and `vpn-bypass clear`; these routing operations require administrator privileges.

## Background service

The service runs:

```text
vpn-bypass watch --interval 60s
```

It starts immediately after installation and automatically starts again after every reboot.

### macOS

Install or upgrade the LaunchDaemon:

```text
sudo vpn-bypass service install
```

The executable is copied to `/usr/local/bin/vpn-bypass`. The LaunchDaemon identifier is `io.github.dimatitov.vpn-bypass`, and logs are written to:

```text
/Library/Logs/vpn-bypass/stdout.log
/Library/Logs/vpn-bypass/stderr.log
```

### Windows

Open PowerShell as Administrator and run:

```powershell
.\vpn-bypass.exe service install
```

The executable is copied to `C:\Program Files\vpn-bypass\vpn-bypass.exe`. Task Scheduler runs `io.github.dimatitov.vpn-bypass` as `SYSTEM` at system startup and restarts it after an unexpected failure.

### Status

Status does not require administrator privileges:

```text
vpn-bypass service status
```

The output is stable and contains exactly one of:

```text
status: not-installed
status: running
status: stopped
```

### Uninstall

On macOS:

```text
sudo vpn-bypass service uninstall
```

On Windows, use an elevated PowerShell terminal:

```powershell
.\vpn-bypass.exe service uninstall
```

Uninstall stops the service, attempts to remove only routes recorded in the vpn-bypass state file, unregisters the service, and removes its installed executable. Cleanup continues if an individual step fails, and all failures are reported together. If Windows is running the installed executable itself, its deletion is scheduled for the next reboot.

User configuration, logs, and any routes not recorded by vpn-bypass are never deleted. Failed route deletions remain in the state file so they can be retried with `vpn-bypass clear`.

## Troubleshooting

Run the routing diagnostic:

```text
vpn-bypass doctor
```

If the service is installed but stopped, inspect its platform status:

```text
launchctl print system/io.github.dimatitov.vpn-bypass
```

On macOS, also inspect the files under `/Library/Logs/vpn-bypass`. On Windows, open Task Scheduler or run:

```powershell
Get-ScheduledTask -TaskName io.github.dimatitov.vpn-bypass
```

Common causes include:

- the VPN changing the direct gateway after startup;
- DNS resolution returning no IPv4 addresses;
- a configuration file that is not valid JSON;
- insufficient privileges for route changes or service installation;
- security software preventing Task Scheduler or route-table changes.

The saved routing state is located at `/Library/Application Support/vpn-bypass/state.json` on macOS and `C:\ProgramData\vpn-bypass\state.json` on Windows. Do not edit it while the watcher is running.
