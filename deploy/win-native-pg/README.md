# Schmutzfink — Windows Server + natives PostgreSQL

Offline-Paket: API, Web-UI, CLIP-Modell, ONNX Runtime und Tokenizer liegen bei. Zielrechner braucht kein Internet für den Start.

Kartenkacheln (OpenStreetMap) und die optionale Adresssuche brauchen weiterhin Netz, falls genutzt.

## Voraussetzungen

- Windows Server x64
- PostgreSQL **16** mit Erweiterung **pgvector**
- TCP-Port **8787** frei (oder in `.env` ändern)

pgvector ist nicht Teil der Standardinstallation. Nach der Postgres-Installation die Erweiterung bereitstellen, danach `postgres\setup.ps1` als Administrator ausführen (psql im PATH).

## Start

1. Zip nach z.B. `C:\Schmutzfink` entpacken.
2. `.env.example` nach `.env` kopieren und Passwörter setzen.
3. `postgres\setup.ps1` ausführen.
4. `.\start.ps1` starten oder `.\install-task.ps1` (Administrator) für Autostart.

UI: http://127.0.0.1:8787 — erstes Konto laut `.env` (`ADMIN_USER` / `ADMIN_PASSWORD`). Passwort `admin` muss beim ersten Login geändert werden.

Fotos liegen unter `data\objects`. Postgres enthält nur Metadaten und Vektoren.
