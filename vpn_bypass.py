#!/usr/bin/env python3
from __future__ import annotations

import argparse
import ipaddress
import json
import os
import platform
import socket
import subprocess
import sys
import time
from pathlib import Path

APP_DIR = Path(__file__).resolve().parent
DOMAINS_FILE = APP_DIR / "domains.txt"
CIDRS_FILE = APP_DIR / "cidrs.txt"
STATE_FILE = APP_DIR / "state.json"
LOG_FILE = APP_DIR / "vpn-bypass.log"


def log(message: str) -> None:
    line = f"{time.strftime('%Y-%m-%d %H:%M:%S')} {message}"
    print(line, flush=True)
    try:
        with LOG_FILE.open("a", encoding="utf-8") as f:
            f.write(line + "\n")
    except OSError:
        pass


def run(cmd: list[str], check: bool = False) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, capture_output=True, check=check)


def require_admin() -> None:
    system = platform.system()
    if system == "Darwin" and os.geteuid() != 0:
        sys.exit("Нужны права root. Запусти через sudo.")
    if system == "Windows":
        cmd = [
            "powershell", "-NoProfile", "-Command",
            "([Security.Principal.WindowsPrincipal]"
            "[Security.Principal.WindowsIdentity]::GetCurrent())."
            "IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)"
        ]
        probe = run(cmd)
        if probe.stdout.strip().lower() != "true":
            sys.exit("Нужны права администратора.")


def read_lines(path: Path) -> list[str]:
    if not path.exists():
        return []
    result: list[str] = []
    for raw in path.read_text(encoding="utf-8").splitlines():
        value = raw.split("#", 1)[0].strip()
        if value:
            result.append(value)
    return result


def resolve_ipv4(hostname: str) -> set[str]:
    ips: set[str] = set()
    try:
        rows = socket.getaddrinfo(hostname, 443, socket.AF_INET, socket.SOCK_STREAM)
        for row in rows:
            ips.add(row[4][0])
    except socket.gaierror as exc:
        log(f"WARN DNS {hostname}: {exc}")
    return ips


def mac_gateway() -> tuple[str, str]:
    result = run(["route", "-n", "get", "default"], check=True)
    gateway = ""
    interface = ""
    for line in result.stdout.splitlines():
        key, _, value = line.strip().partition(":")
        if key == "gateway":
            gateway = value.strip()
        elif key == "interface":
            interface = value.strip()
    if not gateway or not interface:
        raise RuntimeError("Не удалось определить обычный шлюз macOS")
    return gateway, interface


def windows_gateway() -> tuple[str, str]:
    ps = '''
$routes = Get-NetRoute -AddressFamily IPv4 -DestinationPrefix "0.0.0.0/0" |
  Where-Object { $_.NextHop -ne "0.0.0.0" } |
  Sort-Object RouteMetric, InterfaceMetric
foreach ($r in $routes) {
  $a = Get-NetAdapter -InterfaceIndex $r.InterfaceIndex -ErrorAction SilentlyContinue
  if ($a -and $a.Status -eq "Up" -and $a.InterfaceDescription -notmatch "TAP|TUN|OpenVPN|WireGuard|Wintun") {
    [PSCustomObject]@{ Gateway = $r.NextHop; InterfaceIndex = $r.InterfaceIndex } |
      ConvertTo-Json -Compress
    break
  }
}
'''
    result = run(["powershell", "-NoProfile", "-Command", ps], check=True)
    if not result.stdout.strip():
        raise RuntimeError("Не удалось определить обычный шлюз Windows")
    data = json.loads(result.stdout.strip())
    return str(data["Gateway"]), str(data["InterfaceIndex"])


def get_gateway() -> tuple[str, str]:
    system = platform.system()
    if system == "Darwin":
        return mac_gateway()
    if system == "Windows":
        return windows_gateway()
    raise RuntimeError("Поддерживаются только macOS и Windows")


def add_route(target: str, gw: str, iface: str) -> None:
    network = ipaddress.ip_network(target, strict=False)
    system = platform.system()

    if system == "Darwin":
        if network.prefixlen == 32:
            cmd = ["route", "-n", "add", "-host", str(network.network_address), gw]
        else:
            cmd = ["route", "-n", "add", "-net", str(network), gw]
    else:
        ps = (
            f'New-NetRoute -DestinationPrefix "{network}" '
            f'-InterfaceIndex {iface} -NextHop "{gw}" '
            f'-RouteMetric 1 -PolicyStore ActiveStore -ErrorAction Stop'
        )
        cmd = ["powershell", "-NoProfile", "-Command", ps]

    result = run(cmd)
    combined = (result.stdout + result.stderr).lower()
    if result.returncode == 0:
        log(f"ADD {network} via {gw}")
    elif "exists" in combined:
        log(f"KEEP {network}")
    else:
        log(f"ERROR add {network}: {(result.stderr or result.stdout).strip()}")


def delete_route(target: str, iface: str | None = None) -> None:
    network = ipaddress.ip_network(target, strict=False)
    system = platform.system()

    if system == "Darwin":
        if network.prefixlen == 32:
            cmd = ["route", "-n", "delete", "-host", str(network.network_address)]
        else:
            cmd = ["route", "-n", "delete", "-net", str(network)]
    else:
        ps = f'Remove-NetRoute -DestinationPrefix "{network}" -Confirm:$false -ErrorAction SilentlyContinue'
        if iface:
            ps += f" -InterfaceIndex {iface}"
        cmd = ["powershell", "-NoProfile", "-Command", ps]

    result = run(cmd)
    if result.returncode == 0:
        log(f"DEL {network}")


def load_state() -> dict:
    try:
        return json.loads(STATE_FILE.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {"routes": [], "gateway": None, "interface": None}


def save_state(routes: set[str], gw: str, iface: str) -> None:
    STATE_FILE.write_text(
        json.dumps(
            {"routes": sorted(routes), "gateway": gw, "interface": iface},
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )


def desired_routes() -> set[str]:
    routes: set[str] = set()
    for domain in read_lines(DOMAINS_FILE):
        for ip in resolve_ipv4(domain):
            routes.add(f"{ip}/32")

    for value in read_lines(CIDRS_FILE):
        try:
            routes.add(str(ipaddress.ip_network(value, strict=False)))
        except ValueError:
            log(f"WARN неправильный IP/CIDR: {value}")
    return routes


def sync() -> None:
    gw, iface = get_gateway()
    state = load_state()
    old_routes = set(state.get("routes", []))
    wanted = desired_routes()

    if state.get("gateway") != gw or state.get("interface") != iface:
        for route in old_routes:
            delete_route(route, state.get("interface"))
        old_routes = set()

    for route in sorted(old_routes - wanted):
        delete_route(route, iface)

    for route in sorted(wanted - old_routes):
        add_route(route, gw, iface)

    save_state(wanted, gw, iface)
    log(f"SYNC routes={len(wanted)} gateway={gw} interface={iface}")


def clear() -> None:
    state = load_state()
    for route in state.get("routes", []):
        delete_route(route, state.get("interface"))
    try:
        STATE_FILE.unlink()
    except FileNotFoundError:
        pass
    log("CLEAR done")


def status() -> None:
    print(json.dumps(load_state(), ensure_ascii=False, indent=2))


def watch(interval: int) -> None:
    log(f"WATCH started interval={interval}s")
    while True:
        try:
            sync()
        except Exception as exc:
            log(f"ERROR sync: {exc}")
        time.sleep(interval)


def main() -> None:
    parser = argparse.ArgumentParser(description="Домены/IP мимо full-tunnel OpenVPN")
    subs = parser.add_subparsers(dest="command", required=True)
    subs.add_parser("sync")
    subs.add_parser("clear")
    subs.add_parser("status")
    watch_parser = subs.add_parser("watch")
    watch_parser.add_argument("--interval", type=int, default=60)
    args = parser.parse_args()

    if args.command != "status":
        require_admin()

    if args.command == "sync":
        sync()
    elif args.command == "clear":
        clear()
    elif args.command == "status":
        status()
    else:
        watch(max(args.interval, 15))


if __name__ == "__main__":
    main()
