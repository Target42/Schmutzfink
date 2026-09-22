#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if [ "$(id -u)" -ne 0 ]; then
  echo "Bitte als root ausführen: sudo $0" >&2
  exit 1
fi

INSTALL_DIR="${INSTALL_DIR:-/opt/schmutzfink}"
if [ ! -x "$INSTALL_DIR/schmutzfink" ]; then
  echo "Keine Installation unter $INSTALL_DIR — zuerst install.sh ausführen." >&2
  exit 1
fi
if [ ! -x ./schmutzfink ]; then
  echo "Neues Binary fehlt im aktuellen Ordner." >&2
  exit 1
fi

if systemctl is-active --quiet schmutzfink.service 2>/dev/null; then
  systemctl stop schmutzfink.service
fi

cp -a schmutzfink "$INSTALL_DIR/schmutzfink"
chmod +x "$INSTALL_DIR/schmutzfink"
chown schmutzfink:schmutzfink "$INSTALL_DIR/schmutzfink"

systemctl daemon-reload
systemctl start schmutzfink.service
echo "Update fertig. Modelle und Fotos unter $INSTALL_DIR bleiben unangetastet."
echo "Schmutzfink läuft: http://127.0.0.1:8787"
