# Schmutzfink — Linux, Docker Compose

Offline-Paket: App-Image (inkl. CLIP-Modell) und Postgres-Image als Tar, plus Compose-Datei.

## Voraussetzungen

- Linux x86_64
- Docker Engine und Docker Compose Plugin
- Port **8787** lokal frei (von außen reicht 80/443 über nginx)

## Start

```bash
sudo bash ./load-and-start.sh
```

Das Skript lädt `images/*.tar` (falls vorhanden) und startet den Stack. Ohne Tars wird `docker compose build` versucht — das braucht Basis-Images und Netz.

UI zum Test: http://127.0.0.1:8787 — Konto in `.env` bzw. Compose-Umgebung. Passwort `admin` muss beim ersten Login geändert werden, sonst bleibt der Bestand gesperrt.

## Hinter nginx (Port 80/443)

Der App-Port ist nur lokal veröffentlicht:

```yaml
ports:
  - "127.0.0.1:8787:8787"
```

Im Container bleibt `HTTP_ADDR=:8787`, `TRUST_PROXY=1` (Docker-NAT). Hinter nginx `COOKIE_SECURE=auto`.

Beispiel-vHost: `nginx/schmutzfink.conf` (`server_name` und Zertifikate anpassen, `client_max_body_size 110m`). `COOKIE_SECURE=auto` steht in `.env`.

Daten:

- Postgres: Volume `pgdata`
- Fotos: Volume `appdata` (oder S3/MinIO über `STORAGE_BACKEND=s3` und `S3_*`)

Kartenkacheln von OpenStreetMap brauchen Internetzugang im Browser, die Bildsuche nicht.

Active Directory: `LDAP_URL`, `LDAP_DOMAIN` bzw. `LDAP_BIND_TEMPLATE` und optional `PROVISION_TOKEN` in der Compose-Umgebung setzen. Der Container muss den Domain Controller erreichen. Details im Haupt-README, Abschnitt „Active Directory“.
