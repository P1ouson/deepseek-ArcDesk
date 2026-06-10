#!/usr/bin/env bash
# Install ArcDesk desktop into the current user's home (no root required).
set -euo pipefail

APP_NAME="ArcDesk"
BIN_NAME="arcdesk-desktop"
ROOT="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/arcdesk"
BIN_DIR="${XDG_BIN_HOME:-$HOME/.local/bin}"
DESKTOP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"

mkdir -p "$INSTALL_DIR" "$BIN_DIR" "$DESKTOP_DIR"
install -m 0755 "$ROOT/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
ln -sf "$INSTALL_DIR/$BIN_NAME" "$BIN_DIR/$BIN_NAME"

cat >"$DESKTOP_DIR/arcdesk.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=$APP_NAME
Comment=DeepSeek-native coding agent desktop
Exec=$BIN_DIR/$BIN_NAME
Icon=$INSTALL_DIR/appicon.png
Terminal=false
Categories=Development;IDE;
StartupWMClass=arcdesk
EOF

if [ -f "$ROOT/appicon.png" ]; then
  install -m 0644 "$ROOT/appicon.png" "$INSTALL_DIR/appicon.png"
fi

echo "Installed $APP_NAME to $INSTALL_DIR"
echo "Launch: $BIN_NAME   (ensure $BIN_DIR is on PATH)"
