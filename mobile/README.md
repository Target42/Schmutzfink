# Schmutzfink Mobile

Flutter-Client für Außendienst: **Kamera oder Galerie** → Upload an die Go-API.
Kamera nutzt Live-GPS (mit Genauigkeitsanzeige); bei Fehlschlag oder Wunsch Ort auf der Karte.
Galerie: EXIF-GPS oder manuell auf der Karte (ohne EXIF startet die Karte am aktuellen Standort). Optional Notiz, Tags und Vorgang.
Mit GPS-Fix zeigt die App **Funde in der Nähe** (Motiv, Abstand, letztes Foto). Tippen hängt das neue Foto an dieses Motiv und übernimmt einen offenen Vorgang. Ist das letzte Foto eines Motivs 90 Tage oder älter, gibt es den Hinweis auf ein Kontrollfoto.
Nach einer Einzelaufnahme kann ein **Ausschnitt** gezogen werden. Hochgeladen wird das ganze Foto; der Ausschnitt gilt für die Ähnlichkeitssuche.
**Pflichtfelder** aus der Felddefinition füllt die App schon bei der Aufnahme. Sie gelten zusammen mit Notiz, Tags, Vorgang und Motiv für eine Serie.
Serie: mehrere Galerie-Fotos oder Kamera-Wiederholung mit gleichen Metadaten. Eine Galerie-Serie hat keinen Ausschnitt je Foto.
Jedes Foto geht zuerst in die lokale Warteschlange; der Upload läuft danach im Hintergrund (ohne die nächste Aufnahme zu blockieren). Zusätzlich drainiert die Queue bei App-Resume und wenn das Netz wieder da ist.
Letzte Uploads öffnen den Datensatz in der Web-UI.

Keine zweite Voll-UI. Liste, Karte, Vorgänge und Admin bleiben in der Web-App.

## Voraussetzungen

- Flutter SDK (stable)
- Laufende Schmutzfink-API (`go run ./cmd/server`, Port **8787**)
- Sachbearbeiter- oder Admin-Konto (Sucher können nicht hochladen)

## Starten

```powershell
cd mobile
flutter pub get
flutter run
```

### Server-URL

Beim ersten Start ist die Server-URL **leer** — unter **Server einstellen** eintragen:

| Gerät | Typische URL |
|-------|----------------|
| Android-Emulator | `http://10.0.2.2:8787` |
| iOS-Simulator | `http://127.0.0.1:8787` |
| Physisches Handy | `http://<LAN-IP-des-PCs>:8787` |

Für Dev-Builds optional vorausfüllen: `--dart-define=DEFAULT_BASE_URL=http://10.0.2.2:8787`

Die Sitzung läuft über das Cookie `sf_session` (wie die Web-UI), nicht über API-Token (die sind nur lesend).

## Ablauf

1. Anmelden
2. Mit GPS: Funde in der Nähe prüfen, bei Bedarf antippen (Motiv und offener Vorgang)
3. Foto **aufnehmen** (Live-GPS ± m) oder aus der **Galerie** / **Serie** wählen
4. Optional: Ausschnitt, Ort auf Karte, Notiz, Tags, Vorgang, Pflichtfelder
5. Zur Warteschlange → sofort weiter fotografieren; Upload im Hintergrund via `POST /api/records` (Retry bei Netzfehlern / Netz-Wechsel)
6. Unten: **Warteschlange** und **Letzte Uploads** — Tippen öffnet `/datensatz/…` im Browser

Ohne GPS bleibt der Datensatz gültig; der Ort kann später in der Web-UI nachgezogen werden.

## Build (Android APK)

### Release-Signatur (einmalig)

Ohne eigenen Keystore signiert der Release-Build weiterhin mit dem Debug-Key — **nicht** für Verteilung.

1. Keystore anlegen (Passwörter sicher ablegen, Datei backupen):

```powershell
cd mobile\android
keytool -genkey -v `
  -keystore schmutzfink-upload.jks `
  -storetype PKCS12 `
  -keyalg RSA -keysize 2048 `
  -validity 10000 `
  -alias schmutzfink
```

2. `key.properties.example` nach `key.properties` kopieren und Passwörter eintragen:

```powershell
copy key.properties.example key.properties
```

`schmutzfink-upload.jks` und `key.properties` liegen unter `android/` und sind per `.gitignore` ausgeschlossen. **Denselben Keystore für alle späteren Updates verwenden.**

### APK bauen

```powershell
cd mobile
flutter build apk --release
```

Ausgabe: `build/app/outputs/flutter-apk/app-release.apk`

Signatur prüfen (Android SDK `build-tools`, `apksigner` im PATH):

```powershell
apksigner verify --print-certs build\app\outputs\flutter-apk\app-release.apk
```

### Portal-Download

APK und Metadaten für das Web-Portal ablegen (`MOBILE_APK_PATH`, Standard `./data/mobile/schmutzfink-android.apk`):

```powershell
cd mobile
.\scripts\publish-apk.ps1
```

Danach im Portal unter **Konto → Android-App** (`/app`) für Sachbearbeiter und Admins sichtbar. Optional `notes` in `data/mobile/schmutzfink-android.json` setzen. Server neu starten ist nicht nötig — die Datei wird bei jedem Download gelesen.