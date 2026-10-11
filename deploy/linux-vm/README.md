# Schmutzfink — Linux-VM (systemd + natives PostgreSQL)

Offline-Paket: Linux-Binary, Web-UI, CLIP-Modell, ONNX Runtime, Tokenizer, systemd-Unit.

## Voraussetzungen

- Linux x86_64 (glibc, z.B. Debian/Ubuntu)
- PostgreSQL **16** mit **pgvector** (`postgresql-16-pgvector` o.ä.)
- systemd
- Port **8787** lokal frei (von außen reicht 80/443 über nginx)

## Installation

Als root:

```bash
sudo bash ./install.sh
```

Das Skript kopiert nach `/opt/schmutzfink`, legt Rolle/Datenbank an (falls `psql` da ist) und aktiviert `schmutzfink.service`.

UI zum Test: http://127.0.0.1:8787

Konfiguration: `/opt/schmutzfink/.env` (Passwort `admin` oder `CHANGE_ME` muss beim ersten Login geändert werden).  
Fotos: `/opt/schmutzfink/data/objects`

Logs: `journalctl -u schmutzfink -f`

## Update

Die Web-UI steckt im Binary. CLIP/ONNX unter `vendor/` und Fotos unter `data/objects` bleiben.

Auf dem Entwicklungsrechner (Windows), nur das Linux-Binary:

```powershell
.\scripts\build-linux.ps1
```

Danach reicht die eine Datei `dist/cache/bin/schmutzfink` (kein ZIP, keine Modelle):

```bash
scp dist/cache/bin/schmutzfink root@IHRE-SERVER-IP:/tmp/schmutzfink
ssh root@IHRE-SERVER-IP
sudo systemctl stop schmutzfink
sudo cp /tmp/schmutzfink /opt/schmutzfink/schmutzfink
sudo chmod +x /opt/schmutzfink/schmutzfink
sudo chown schmutzfink:schmutzfink /opt/schmutzfink/schmutzfink
sudo systemctl start schmutzfink
```

Oder das Binary neben `update.sh` legen und `sudo bash ./update.sh` ausführen.

`install.sh` ist nur für die Erstinstallation (kopiert auch die großen Modelldateien). Schema-Änderungen macht die App beim Start selbst.

Kontrolle: `systemctl status schmutzfink` und `journalctl -u schmutzfink -n 50`.

## Hinter nginx (Port 80/443)

Die API bleibt auf 8787, nginx terminiert TLS und reicht weiter. In `/opt/schmutzfink/.env`:

```
HTTP_ADDR=127.0.0.1:8787
COOKIE_SECURE=auto
TRUST_PROXY=auto
```

Beispiel-vHost liegt bei `nginx/schmutzfink.conf` (`server_name` und Zertifikate anpassen). Wichtig: `client_max_body_size 110m` (ZIP-Import bis 100 MB). Danach:

```bash
sudo systemctl restart schmutzfink
sudo nginx -t && sudo systemctl reload nginx
```
