#!/bin/bash
set -euo pipefail
TARGET_DIR="/usr/local/lib/vpn-bypass"
PLIST="/Library/LaunchDaemons/local.vpn-bypass.plist"

if [[ -f "$TARGET_DIR/vpn_bypass.py" ]]; then
  sudo python3 "$TARGET_DIR/vpn_bypass.py" clear || true
fi
sudo launchctl bootout system "$PLIST" 2>/dev/null || true
sudo rm -f "$PLIST"
sudo rm -rf "$TARGET_DIR"
echo "Удалено."
