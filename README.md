# Schmutzfink

Inventar für Graffiti-Fotos: Upload, Bestand, Karte, interne Anmeldung, Bildähnlichkeit.

Clients sprechen nur mit der Go-API. Die Web-UI ist der erste Client.

## Starten (Windows)

Postgres lauscht auf Host-Port **55432** (nicht 5432/5433), damit es nicht mit einer lokalen Installation kollidiert. Das Image ist `pgvector/pgvector:pg16` (Vektoren für die Bildsuche).

```powershell
docker compose up -d
```

Beim Wechsel vom alten `postgres:16`-Image reicht in der Regel ein erneutes `docker compose up -d` — die Daten bleiben. Fehlt die Erweiterung `vector`, Container neu anlegen (`docker compose down` ohne `-v`, dann `up -d`).

2. API (Port **8787**, weil 8080 oft schon belegt ist):

```powershell
go run ./cmd/server
```

Beim **ersten Start** lädt die API ONNX Runtime und das CLIP-Modell (rund 600 MB, Cache unter dem Benutzerprofil). Die Web-UI ist sofort nutzbar; Bildsuche und „ähnliche Bilder“ sind bereit, sobald in der Konsole `CLIP/ONNX: bereit` steht. Abschalten: `EMBEDDINGS=0`.

3. Web-UI (zweites Terminal):

```powershell
cd web
npm install
npm run dev
```

Browser: [http://localhost:5173](http://localhost:5173)

Erstes Konto aus `.env`: `ADMIN_USER` / `ADMIN_PASSWORD` (nur wenn die Datenbank noch leer ist). Standardpasswort `admin` wird beim ersten Login zwingend geändert — ohne neues Passwort gibt es keinen Zugriff auf Fotos und Metadaten. Danach legt der Admin unter **Benutzer** Konten an: **Sachbearbeiter** (Bestand pflegen), **Sucher** (nur suchen, auch mit Vergleichsbild) und bei Bedarf weitere Admins. **Löschbeauftragter** ist eine Zusatzrolle — nur sie führen beantragte Löschungen aus, nicht den eigenen Antrag. Unter **Felder** definiert und ändert der Admin eigene Metadaten (Text, Zahl, Datum, Ja/Nein, Auswahl; Typ und Schlüssel bleiben fest); Sachbearbeiter füllen sie an der Sichtung. Die Textsuche findet die Werte mit, und in Liste und Karte gibt es je Feld einen Filter. Unter **Auswertungen** gibt es Zählungen, Zeitverlauf und Hotspots zu denselben Filtern. Unter **Protokoll** sieht der Admin Anmeldungen, Imports und Änderungen.

## Was v1 kann

- Login (Cookie-Sitzung). Standardpasswort reicht nicht: der Bestand bleibt gesperrt, bis ein eigenes Passwort gesetzt ist. Optional **Active Directory**: `LDAP_URL` plus `LDAP_DOMAIN` oder `LDAP_BIND_TEMPLATE`; das Kennwort wird per LDAP-Bind geprüft, Rollen bleiben in Schmutzfink. Der AD-Aufnahmeprozess legt Konten über `POST /api/provision/users` an (`Authorization: Bearer` mit `PROVISION_TOKEN`, mindestens 16 Zeichen). Unter **API-Token** (Konto-Menü) gibt es einen lesenden Maschinen-Zugang: `Authorization: Bearer sft_…` für Suche, Karte und Export, ohne Bilder und ohne Schreibrechte.
- Bild hochladen (JPEG/PNG/WebP/GIF), Original + Thumbnail hinter einem Storage-Interface
- EXIF: GPS und Aufnahmezeit, sonst manuell nachziehbar
- Liste, Detail, Karte
- Filter: Text (Notiz/Tags/eigene Felder/Motivtitel/Vorgang), Zeitraum, Umkreis, mit/ohne GPS, Motiv, Vorgang, plus Filter je eigenem Feld
- Export der aktuellen Treffer als CSV oder JSON (gleiche Filter, ohne Bilddateien)
- **Auswertungen**: Zählungen, Zeitverlauf, Tags, Motive, Felder und Hotspots zu denselben Filtern

## Was v2 kann

- Asynchrones CLIP-Embedding nach dem Upload (Go + ONNX, kein Python)
- Ähnliche Bilder auf der Detailseite
- Suche im Modus **Bildinhalt** (Freitext über den CLIP-Textencoder)
- Filter (Zeit, GPS, Umkreis) gelten auch für KI-Treffer

## Was v4 kann

- Sichtungen manuell einem **Motiv** zuordnen und wieder lösen; in Liste und Karte nach Motiv filtern
- Motiv-Ansicht: Zeitreihe und Karte der Fundorte
- CLIP-Vorschläge „ähnliche Sichtung diesem Motiv zuordnen“ — nur nach Bestätigung
- Kein automatisches Zusammenlegen

## Vorgänge und Löschfristen

- Fotos einem **Vorgang** zuordnen: zivilrechtlich 3 Jahre, strafrechtlich 5 Jahre — Frist ab dem Schließen durch einen Bearbeiter
- Nach Ablauf: GPS und Adresse inklusive PLZ werden entfernt, das Foto bleibt
- Sachbearbeiter können weiterhin eine vollständige Löschung beantragen

Bilder liegen lokal unter `STORAGE_DIR` (Standard `data/objects`) als `Jahr/Monat` plus Hex-Shard, oder in S3/MinIO (`STORAGE_BACKEND=s3`). In Postgres nur Object-IDs und `vector(512)`. MinIO zum Testen: `docker compose --profile s3 up -d` (Image `ghcr.io/coollabsio/minio`, Konsole http://127.0.0.1:9001).

## Active Directory

Schmutzfink speichert Rollen lokal. Das AD prüft nur das Kennwort (LDAP-Simple-Bind). Es gibt keine Gruppensynchronisation und kein SSO-Redirect. Ohne `LDAP_URL` bleiben nur lokale Konten.

### 1. Netz und Zertifikat

Der **API-Prozess** (bzw. der Container) muss den Domain Controller erreichen, nicht der Browser.

- `ldaps://dc.example.local:636` — TLS ab dem Verbindungsaufbau (bevorzugt).
- `ldap://dc.example.local:389` plus `LDAP_STARTTLS=1` — Klartext, dann Upgrade.
- Eigenes oder internes CA-Zertifikat gehört in den Trust Store des Hosts bzw. Containers. `LDAP_INSECURE_SKIP_VERIFY=1` nur zum Test, nicht im Betrieb.

Hinter Docker muss der DC vom Container aus auflösbar sein (DNS der Domäne oder feste IP in `LDAP_URL`).

### 2. Bind-Identität

Beim Login setzt Schmutzfink den eingegebenen Namen in eine AD-Bind-Identität ein.

| Einstellung | Bind als | Wann |
| --- | --- | --- |
| `LDAP_DOMAIN=example.local` (enthält einen Punkt) | `j.mueller@example.local` | UPN, der Normalfall |
| `LDAP_DOMAIN=STADT` (ohne Punkt) | `STADT\j.mueller` | NetBIOS / `DOMAIN\user` |
| `LDAP_BIND_TEMPLATE={username}@example.local` | wie die Vorlage | überschreibt `LDAP_DOMAIN` |

Steht im Benutzernamen schon ein `@` oder `\`, wird die Vorlage nicht mehr angewandt.

Der **Benutzername in Schmutzfink** muss zum AD-Konto passen, in der Regel `sAMAccountName` (`j.mueller`), nicht der Anzeigename.

### 3. Umgebung

In `.env` bzw. Compose (`deploy/linux-compose` reicht `LDAP_*` und `PROVISION_TOKEN` durch):

```
LDAP_URL=ldaps://dc.example.local:636
LDAP_DOMAIN=example.local
# optional, statt LDAP_DOMAIN:
# LDAP_BIND_TEMPLATE={username}@example.local
# LDAP_STARTTLS=0
# LDAP_INSECURE_SKIP_VERIFY=0
PROVISION_TOKEN=mindestens-16-zufaellige-zeichen
```

Nach dem Setzen API neu starten. In der Konsole erscheint `anmeldung: AD/LDAP …`. Ein lokales Admin-Konto behalten — bei AD-Ausfall bleibt der Notzugang.

### 4. Konten anlegen

AD-User existieren in Schmutzfink erst, wenn sie angelegt wurden. Zwei Wege:

- UI **Benutzer** → Konto „Active Directory“, Name wie im Verzeichnis, Rolle setzen. Kein Passwort.
- Der bestehende AD-Aufnahmeprozess ruft die Provisioning-API auf (`PROVISION_TOKEN`, mindestens 16 Zeichen; leer = aus, Antwort dann 404). Das ist **nicht** dasselbe wie die lesenden `sft_`-Token.

```http
POST /api/provision/users
Authorization: Bearer <PROVISION_TOKEN>
Content-Type: application/json

{"username":"j.mueller","role":"user","can_delete":false,"external_id":""}
```

`external_id` ist optional (z. B. objectGUID). Rollen: `user`, `searcher`, `admin`. `can_delete` ist das Flag Löschbeauftragter (bei `searcher` wirkungslos).

```http
GET /api/provision/users/j.mueller
Authorization: Bearer <PROVISION_TOKEN>
```

```http
PATCH /api/provision/users/j.mueller
Authorization: Bearer <PROVISION_TOKEN>
Content-Type: application/json

{"disabled":true}
```

`PATCH` kann auch `role` und `can_delete` setzen. Beim Austritt aus dem AD denselben Prozess `disabled: true` schicken — sonst bleibt das Schmutzfink-Konto aktiv, bis jemand es in der UI sperrt.

### 5. Login und typische Fehler

Die Person gibt denselben Namen und das **AD-Kennwort** ein. Schmutzfink bindet gegen den DC; bei Erfolg gibt es die normale Cookie-Sitzung. Passwort ändern nur im AD. Lokale Konten funktionieren parallel.

| Symptom | Übliche Ursache |
| --- | --- |
| `AD-Anmeldung ist nicht eingerichtet` | `LDAP_URL` fehlt oder API nicht neu gestartet |
| `Benutzername oder Passwort falsch` | Konto in Schmutzfink fehlt, Name ≠ `sAMAccountName`, falsches Kennwort, oder Bind-Vorlage passt nicht zur Domäne |
| Timeout / 500 beim Login | DC vom API-Host nicht erreichbar, DNS, Firewall 389/636 |
| Zertifikatsfehler | CA nicht im Trust Store; `LDAP_URL` Host muss zum Zertifikat-CN/SAN passen |

## Hinter nginx

Die API bleibt auf Port **8787**, standardmäßig nur auf localhost. nginx nimmt 80/443 entgegen und reicht an `127.0.0.1:8787` weiter. In `.env`:

```
HTTP_ADDR=127.0.0.1:8787
COOKIE_SECURE=auto
TRUST_PROXY=auto
```

Beispiel: `deploy/nginx/schmutzfink.conf` — `server_name` und Zertifikate anpassen. `client_max_body_size` muss mindestens **110m** sein (ZIP-Import).
