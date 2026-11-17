#!/usr/bin/env bash
set -euo pipefail

SUPER="${PGUSER:-postgres}"

psql_super() {
  if [ "$(id -u)" -eq 0 ] && id postgres >/dev/null 2>&1; then
    sudo -u postgres psql -v ON_ERROR_STOP=1 "$@"
  else
    psql -U "$SUPER" -v ON_ERROR_STOP=1 "$@"
  fi
}

if ! command -v psql >/dev/null 2>&1; then
  echo "psql fehlt. Unter Debian/Ubuntu z.B.: apt install postgresql postgresql-16-pgvector" >&2
  exit 1
fi

psql_super -d postgres -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'schmutzfink') THEN CREATE ROLE schmutzfink LOGIN PASSWORD 'schmutzfink'; END IF; END \$\$;"

exists="$(psql_super -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = 'schmutzfink'" | tr -d '[:space:]')"
if [ "$exists" != "1" ]; then
  psql_super -d postgres -c "CREATE DATABASE schmutzfink OWNER schmutzfink;"
fi

psql_super -d schmutzfink -c "CREATE EXTENSION IF NOT EXISTS vector;"
psql_super -d schmutzfink -c "GRANT ALL ON SCHEMA public TO schmutzfink;"
echo "Postgres ist bereit (Datenbank schmutzfink, Erweiterung vector)."
