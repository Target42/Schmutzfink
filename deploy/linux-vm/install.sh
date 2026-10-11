#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if [ "$(id -u)" -ne 0 ]; then
  echo "Bitte als root ausführen: sudo $0" >&2
  exit 1
fi

INSTALL_DIR="${INSTALL_DIR:-/opt/schmutzfink}"
id schmutzfink >/dev/null 2>&1 || useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin schmutzfink

mkdir -p "$INSTALL_DIR"
if systemctl is-active --quiet schmutzfink.service 2>/dev/null; then
  systemctl stop schmutzfink.service
fi

cp -a schmutzfink "$INSTALL_DIR/schmutzfink"
chmod +x "$INSTALL_DIR/schmutzfink"
cp -a web "$INSTALL_DIR/"
cp -a vendor "$INSTALL_DIR/"
if [ ! -f "$INSTALL_DIR/.env" ]; then
  if [ -f .env ]; then
    cp -a .env "$INSTALL_DIR/.env"
  elif [ -f .env.example ]; then
    cp -a .env.example "$INSTALL_DIR/.env"
  fi
else
  echo "Bestehende $INSTALL_DIR/.env bleibt unverändert."
fi
mkdir -p "$INSTALL_DIR/data/objects"
chown -R schmutzfink:schmutzfink "$INSTALL_DIR"

if [ -x ./postgres/setup.sh ]; then
  ./postgres/setup.sh || echo "Postgres-Setup übersprungen — bitte postgres/setup.sh manuell ausführen."
fi

install -m 0644 schmutzfink.service /etc/systemd/system/schmutzfink.service
systemctl daemon-reload
systemctl enable --now schmutzfink.service
echo "Schmutzfink läuft: http://127.0.0.1:8787"
