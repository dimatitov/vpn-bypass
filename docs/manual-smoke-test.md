# v0.1.0 manual smoke test

Use release archives produced from the candidate commit. Start with a VPN that routes ordinary internet traffic through its tunnel.

## macOS

- Download the archive matching the Mac architecture and verify its SHA-256 value against `checksums.txt`.
- Extract it and confirm `./vpn-bypass version` reports `v0.1.0`, the full commit SHA, and a UTC RFC3339 date.
- Run `sudo ./vpn-bypass install`; confirm the binary, configuration, LaunchDaemon plist, and log directory exist.
- Confirm an existing configuration is unchanged after running install a second time.
- Confirm `vpn-bypass service status` reports `running` and remains running after reboot.
- Run elevated `vpn-bypass doctor`; confirm all checks pass with the VPN connected.
- Add and remove a test domain, run `list`, and confirm the JSON configuration remains editable.
- Run `sync`, `status`, `logs`, and `logs --follow`; verify managed routes and timestamps update.
- Run `sudo vpn-bypass uninstall`; confirm the daemon and installed binary are removed while configuration and logs remain.
- Reinstall, then run `sudo vpn-bypass uninstall --purge`; confirm configuration, state, and logs are removed.

## Windows

- Download the Windows archive and verify its SHA-256 value against `checksums.txt`.
- Extract it and confirm `vpn-bypass.exe version` reports `v0.1.0`, the full commit SHA, and a UTC RFC3339 date.
- From elevated PowerShell, run `.\vpn-bypass.exe install`; confirm the Program Files binary, ProgramData configuration, log directory, and SYSTEM task exist.
- Confirm installation does not modify machine or user `PATH`.
- Confirm an existing configuration is unchanged after running install a second time.
- Confirm `service status` reports `running` and the task starts after reboot.
- Run elevated `doctor`; confirm all checks pass with the VPN connected.
- Exercise `add`, `remove`, `list`, `sync`, `status`, `logs`, and `logs --follow` using the installed path containing spaces.
- Run `uninstall`; confirm the task is removed and configuration/logs remain. Reboot if executable deletion is reported as pending.
- Reinstall, then run `uninstall --purge`; confirm configuration, state, and logs are removed.
