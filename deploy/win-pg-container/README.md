# Schmutzfink — Windows Server + Postgres-Container

Offline-Paket: API, Web-UI, CLIP-Modell und (falls mitgepackt) das Docker-Image `pgvector/pgvector:pg16`.

## Voraussetzungen

- Windows Server x64
- Docker Engine / Docker Desktop
- TCP-Port **8787** (API) und **5432** (Postgres-Container) frei

## Start

1. Zip nach z.B. `C:\Schmutzfink` entpacken.
2. `.env.example` nach `.env` kopieren und Passwörter setzen.
3. `.\load-postgres.ps1` — lädt das Image aus `images\`, falls vorhanden, sonst `docker compose pull`.
4. `docker compose up -d`
5. `.\start.ps1` oder `.\install-task.ps1` (Administrator) für Autostart der API.

UI: http://127.0.0.1:8787 — Passwort `admin` muss beim ersten Login geändert werden.

Postgres-Daten liegen im Docker-Volume `pgdata`. Fotos unter `data\objects`.
