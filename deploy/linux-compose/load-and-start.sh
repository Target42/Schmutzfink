#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker fehlt." >&2
  exit 1
fi

load_tar() {
  local tar="$1"
  if [ -f "$tar" ]; then
    echo "Lade $tar ..."
    docker load -i "$tar"
  fi
}

load_tar images/pgvector-pg16.tar
load_tar images/schmutzfink.tar

if ! docker image inspect schmutzfink:offline >/dev/null 2>&1; then
  echo "App-Image fehlt, baue lokal ..."
  docker compose build
fi

docker compose up -d
echo "Schmutzfink läuft: http://127.0.0.1:8787"
