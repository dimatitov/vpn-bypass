#!/bin/bash
set -euo pipefail

SOURCE_DIR="$(cd "$(dirname "$0")" && pwd)"
TARGET_DIR="/usr/local/lib/vpn-bypass"
PLIST="/Library/LaunchDaemons/local.vpn-bypass.plist"
PYTHON="$(command -v python3 || true)"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "Этот установщик только для macOS."; exit 1
fi
if [[ -z "$PYTHON" ]]; then
  echo "Не найден python3. Установи: brew install python"; exit 1
fi

sudo mkdir -p "$TARGET_DIR"
sudo cp "$SOURCE_DIR/vpn_bypass.py" "$SOURCE_DIR/domains.txt" "$SOURCE_DIR/cidrs.txt" "$TARGET_DIR/"
sudo chmod 755 "$TARGET_DIR/vpn_bypass.py"
sudo chmod 644 "$TARGET_DIR/domains.txt" "$TARGET_DIR/cidrs.txt"

sudo tee "$PLIST" >/dev/null <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>local.vpn-bypass</string>
  <key>ProgramArguments</key>
  <array>
    <string>$PYTHON</string>
    <string>$TARGET_DIR/vpn_bypass.py</string>
    <string>watch</string>
    <string>--interval</string>
    <string>60</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>$TARGET_DIR/launchd.out.log</string>
  <key>StandardErrorPath</key><string>$TARGET_DIR/launchd.err.log</string>
</dict>
</plist>
EOF

sudo chown root:wheel "$PLIST"
sudo chmod 644 "$PLIST"
sudo launchctl bootout system "$PLIST" 2>/dev/null || true
sudo launchctl bootstrap system "$PLIST"
sudo launchctl enable system/local.vpn-bypass
sudo launchctl kickstart -k system/local.vpn-bypass

echo "Готово."
echo "Домены: sudo nano $TARGET_DIR/domains.txt"
echo "Лог: tail -f $TARGET_DIR/vpn-bypass.log"
