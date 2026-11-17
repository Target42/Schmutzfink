#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
  echo ".env aus .env.example angelegt. Bitte Passwörter anpassen."
fi
exec ./schmutzfink
