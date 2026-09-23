#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

$psql = Get-Command psql -ErrorAction SilentlyContinue
if (-not $psql) {
    throw "psql nicht im PATH. PostgreSQL 16 mit pgvector installieren und dieses Skript erneut ausfuehren."
}

$super = if ($env:PGUSER) { $env:PGUSER } else { 'postgres' }
Write-Host "Lege Rolle und Datenbank an (Superuser: $super) ..."

& $psql.Source -U $super -d postgres -v ON_ERROR_STOP=1 -c @"
DO `$`$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'schmutzfink') THEN
    CREATE ROLE schmutzfink LOGIN PASSWORD 'schmutzfink';
  END IF;
END `$`$;
"@
if ($LASTEXITCODE -ne 0) { throw "Rolle konnte nicht angelegt werden." }

$db = (& $psql.Source -U $super -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = 'schmutzfink'").Trim()
if ($db -ne '1') {
    & $psql.Source -U $super -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE schmutzfink OWNER schmutzfink;"
    if ($LASTEXITCODE -ne 0) { throw "Datenbank konnte nicht angelegt werden." }
}

& $psql.Source -U $super -d schmutzfink -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS vector;"
if ($LASTEXITCODE -ne 0) { throw "pgvector-Erweiterung fehlt. Bitte pgvector fuer PostgreSQL 16 installieren." }

& $psql.Source -U $super -d schmutzfink -v ON_ERROR_STOP=1 -c "GRANT ALL ON SCHEMA public TO schmutzfink;"
Write-Host "Postgres ist bereit (Datenbank schmutzfink, Erweiterung vector)."
