---
marp: true
theme: default
paginate: true
lang: de
title: Schmutzfink — eingesetzte Techniken
---

<!--
Im Editor und auf GitHub ist das normaler Markdown-Text.
Die Erweiterung „Marp for VS Code“ stellt dieselben Abschnitte als Folien dar.
-->

# Was die Technik trägt

Schmutzfink ist ein Inventar für Graffiti-Fotos: Aufnahme, Bestand, Karte, Anmeldung und Bildähnlichkeit. Die Fachlogik liegt in einem Go-Programm. Web, Flutter und Maschinen-Token sind Clients dieser API.

| | |
| --- | --- |
| 1 | API-Prozess |
| 512 | CLIP-Dimensionen |
| 18 | SQL-Migrationen |
| 4 | Auslieferungspakete |

Bilder liegen als Dateien, Metadaten und Vektoren in Postgres. Die Ähnlichkeit rechnet lokal mit OpenCLIP über ONNX Runtime. Ein fehlendes Modell schaltet die Bildsuche ab und lässt Liste, Karte und Textsuche weiterlaufen.

Jede Folie beschreibt eine Technik so, wie sie im Quelltext steht: Grenzen, Speicherort und der Moment, in dem sie greift.

---

# Fünf Leitentscheidungen

Die Architektur ist bewusst eng. Jede Zeile ist eine Entscheidung, die Betrieb, Datenschutz und spätere Clients mitträgt.

| Entscheidung | Umsetzung | Folge |
| --- | --- | --- |
| Ein Backend | HTTP-API, chi | Web und Flutter enthalten keine eigene Fachlogik |
| Go im Betrieb | Ein Binary | HTTP, Worker und Inferenz starten zusammen |
| Postgres | pgx, Migrationen im Binary | Benutzer, Fotos, Vektoren, Audit in einer Datenbank |
| Dateien getrennt | Store-Interface | Platte oder S3, in der DB nur die Object-ID |
| KI lokal | OpenCLIP, ONNX, purego | Bildbytes gehen für die Analyse nicht an einen Dienst |

---

# Ein Prozess, zwei Worker

`cmd/server` verbindet die Datenbank, spielt die Migrationen ein, legt das erste Admin-Konto an, öffnet den Speicher und startet dann drei Dinge nebeneinander: den HTTP-Server, den Embedding-Worker und den Fristen-Worker. CLIP lädt in einer eigenen Goroutine.

```text
Web ────┐
        ├─► Go-API ─┬─ Anmeldung ──► Postgres
Flutter ┘           ├─ Fachdaten ──► Postgres, Dateien
                    ├─ Embedding ──► CLIP, Dateien, Postgres
                    └─ Fristen ────► Postgres
```

Leserichtung von links nach rechts. Die Pfeile sind Aufrufe im selben Prozess, außer dem Objektspeicher, der außerhalb liegt.

- **HTTP.** Adresse aus `HTTP_ADDR`, Standardport 8787. Kopftime 10 Sekunden, Body bis 3 Minuten, Leerlauf 90 Sekunden. SIGINT beendet den Server geordnet.
- **Embedding.** Alle 4 Sekunden und zusätzlich nach einem Upload. Arbeitet nur, wenn das Modell `bereit` meldet.
- **Fristen.** Stündlich und einmal beim Start. Entfernt fällige Standorte und schreibt das ins Protokoll.

---

# Foto, Sichtung, Motiv, Vorgang

Liste, Karte und Ähnlichkeit arbeiten auf Sichtungen. Das Foto ist der Träger von Datei, Ort und Zeit. Ein Motiv entsteht erst, wenn jemand Sichtungen zusammenlegt. Ein Vorgang hängt am Foto und startet die Standortfrist.

| Objekt | Liegt auf | Enthält |
| --- | --- | --- |
| Foto | Tabelle `records` | Datei-IDs, GPS, Aufnahmezeit, Hash, Vorgang |
| Sichtung | Tabelle `sightings` | Rechteck, Notiz, Tags, Felder, Vektor |
| Motiv | Tabelle `motifs` | Titel; Sichtungen verweisen optional darauf |
| Vorgang | Tabelle `cases` | Titel, zivil oder strafrechtlich, Schlusszeit |

Ein Foto ohne gezeichnetes Rechteck hat genau eine Sichtung über das ganze Bild. GPS und Zeitpunkt bleiben am Foto. Ändert sich nur das Rechteck, wird nur diese Sichtung neu eingebettet.

---

# Eine HTTP-API für alle Clients

Der Router ist chi. Jede Antwort bekommt Sicherheitsheader: nosniff, kein Frame, Referrer nur bei Herkunftswechsel, und eine Content-Security-Policy, die Skripte auf die eigene Herkunft begrenzt. Kartenkacheln dürfen von `tile.openstreetmap.org` kommen. Hinter HTTPS setzt der Server HSTS für ein Jahr.

| Zugang | Woran erkannt | Was erlaubt ist |
| --- | --- | --- |
| Öffentliche Karte | Ohne Sitzung, nur wenn freigegeben | Fundorte und kleine Vorschaubilder |
| Browser | Cookie `sf_session` | Rolle entscheidet über Schreiben und Verwaltung |
| Maschinen-Token | `Authorization: Bearer sft_…` | Lesen von Suche, Karte, Export, Zusammenfassung |
| Provisionierung | Eigener Bearer, mindestens 16 Zeichen | Konten anlegen, sperren, Rolle setzen |

Das gebaute Frontend liegt per `go:embed` in `internal/webui/dist` und wird vom selben Server ausgeliefert. In der Entwicklung bleibt Vite auf Port 5173 und spricht die API an.

---

# Web-Oberfläche

Die Oberfläche ist eine React-19-Anwendung mit TypeScript, Vite 8 und React Router 7. Leaflet zeichnet die Karte, `exifr` liest am Client schon GPS und Zeit, damit das Formular vorgefüllt ist. Gespeichert wird erst, was die API aus dem Original liest.

**Im Browser.** Liste, Detail, Karte, Motive, Vorgänge, eigene Felder, Auswertungen, Benutzer, Protokoll. Dieselbe Oberfläche auf Desktop und Handy. oxlint und vitest prüfen Typen und Formatierung der Feldwerte.

**Bewusst außerhalb des Browsers.** Die Policy setzt `camera`, `microphone` und `geolocation` auf leer. Der Browser wird nicht zur Kamera. Aufnehmen mit Geräte-GPS macht die Flutter-App. Styles dürfen inline sein, Skripte und Verbindungen bleiben auf der eigenen Herkunft.

---

# Vom Upload zum Arbeitsbild

Angenommen werden JPEG, PNG, WebP und GIF. Vor dem Decodieren gelten zwei Grenzen: 25 MB und 40 Millionen Pixel. EXIF wird aus dem Rohupload gelesen. Danach wird das Bild für die Ablage normalisiert.

| Schritt | Grenze | Technik |
| --- | --- | --- |
| Typ | JPEG, PNG, WebP, GIF | `http.DetectContentType` |
| Größe | 25 MB, 40 Mio. Pixel | `DecodeConfig` vor dem vollen Bild |
| EXIF | GPS, Aufnahmezeit | goexif, vor dem Verkleinern |
| Arbeitsbild | längste Kante 2048 | Lanczos, sonst Originalbytes |
| Neu kodiert | JPEG-Qualität 85 | Orientierung eingebrannt, EXIF weg |
| Vorschau | 480 px, JPEG 85 | eigene Object-ID |
| Doppelupload | SHA-256 der Bytes | `content_hash` je Mandant |

Ein Rechteck ist auf 0 bis 1 normiert, beide Kanten mindestens 1 Prozent, und muss im Bild liegen. Der Ausschnitt steuert Embedding und Vorschau. Die gespeicherte Datei bleibt das ganze Foto.

---

# Speicher hinter Object-IDs

`storage.Store` kennt nur `Put`, `Open` und `Delete`. Pfade und Bucket-Keys bleiben in diesem Paket. Die Datenbank speichert die ID, die zugleich der Schlüssel ist.

| Backend | Auswahl | Ablage |
| --- | --- | --- |
| Lokal | `STORAGE_BACKEND` leer oder `local` | Verzeichnis, Standard `data/objects` |
| S3 / MinIO | `STORAGE_BACKEND=s3` | AWS SDK v2, eigener Endpoint, Prefix |

Neue IDs haben die Form `Jahr/Monat/aa/bb/uuid`. Die zwei Hex-Paare sind die ersten vier Stellen der UUID und verteilen die Dateien. Alte IDs ohne Schrägstrich bekommen dasselbe Hex-Muster nachträglich. `..` im Schlüssel wird abgewiesen.

Ausschnitt-Vorschaubilder mit Rechteck können zusätzlich in einem lokalen Cache liegen. Der Dateiname ist der SHA-256 aus Sichtung und Rechteck, aufgeteilt in ein Zwei-Zeichen-Verzeichnis.

---

# Postgres, nicht das Dateisystem

Entwicklung und Pakete nutzen `pgvector/pgvector:pg16`. Auf dem Entwicklerrechner lauscht Postgres auf Port 55432, damit eine lokale Installation auf 5432 frei bleibt. Der Pool ist pgx. Beim Start prüft `schema_migrations` die Dateien `001` bis `018` und zieht Fehlendes nach.

| Struktur | Wozu |
| --- | --- |
| `tenants`, `users`, `sessions` | Ein Mandant, Konten, Cookie-Sitzungen |
| `records`, `sightings` | Foto getrennt von Motivausschnitt |
| `sightings.embedding vector(512)` | Ein Vektor je Sichtung |
| HNSW, `vector_cosine_ops` | Nächste Nachbarn nach Cosinus |
| GIN auf `tags` | Tag-Filter |
| Partial Index `pending` | Worker findet offene Embeddings |
| Partial Index GPS | Karte nur über gesetzte Koordinaten |
| `custom_fields`, `sighting_field_values` | Behördliche Felder als Textwert |
| `api_tokens`, `audit` | Maschinen-Zugang und Protokoll |

Die Erweiterung `vector` wird in Migration 002 angelegt. Ab Migration 004 hängt der Vektor an der Sichtung; der alte Index auf `records` ist entfernt.

---

# CLIP im Prozess, ohne Python

Das Modell ist OpenCLIP ViT-B/32 mit Bild- und Textencoder. Beide liefern denselben 512-dimensionalen Vektor, deshalb liegt eine Freitextsuche im selben Raum wie ein Foto. Geladen wird eine Session für die Lebensdauer des Prozesses, über `pure-onnx` und purego, ohne CGO.

**Start.** Liegen Runtime und Modell unter `vendor/`, werden sie von dort geladen. Sonst holt der erste Start die Dateien, rund 600 MB, und legt sie im Benutzerprofil ab. Fehlschläge warten 15 Sekunden und verdoppeln die Pause bis fünf Minuten.

**Aufruf.** Bilder über 1024 Pixel Kante werden vor der Inferenz mit Box-Filter eingepasst. Text wird beschnitten; ein leerer Suchtext ist ein Fehler. `EMBEDDINGS=0` lässt die Engine auf `off`.

Solange der Status nicht `bereit` ist, antwortet die Bildsuche mit „noch nicht bereit“. Liste, Karte, Textfilter und Upload hängen nicht am Modell.

---

# Embeddings neben dem Request

Nach dem Upload ist die Sichtung `pending`. Der Worker beansprucht mit `FOR UPDATE SKIP LOCKED` genau eine Zeile, setzt sie auf `processing` und liest die Datei aus dem Store. Ein Rechteck wird vorher aus dem Bild geschnitten. Der Vektor wird nur geschrieben, wenn `embedding_gen` noch dieselbe Generation ist.

| Zustand | Bedeutung |
| --- | --- |
| `pending` | Wartet, oder nach einem geänderten Rechteck neu |
| `processing` | Vom Worker beansprucht |
| `ready` | Vektor liegt, Suche findet die Sichtung |
| `failed` | Fehlertext, höchstens 500 Zeichen |

Ein Rechteckwechsel erhöht die Generation, leert den Vektor und setzt den Status zurück. Ein Worker, der noch mit der alten Generation rechnet, überschreibt das Ergebnis nicht. Zeilen, die länger als 15 Minuten in `processing` stehen, gehen zurück auf `pending`.

---

# Cosinus im gemeinsamen Raum

pgvector vergleicht mit dem Cosinus-Operator `<=>`. Der angezeigte Score ist 1 minus Distanz: 1 ist identisch, 0 ist orthogonal. Der HNSW-Index nutzt `vector_cosine_ops`. Filter auf Zeit, GPS, Umkreis, Motiv, Vorgang und Felder werden in dieselbe WHERE-Klausel geschrieben wie bei der Liste.

| Modus | Vektorquelle | Treffer |
| --- | --- | --- |
| Ähnliche Bilder | Embedding der Ausgangssichtung | Standardschwelle 0.5, 12 Treffer, höchstens 40 |
| Bildinhalt | Textencoder der Suchanfrage | Dieselben Filter, ein Treffer pro Foto |

Pro Foto bleibt die beste Sichtung (`DISTINCT ON`, sortiert nach Distanz). Bei „ähnliche Bilder“ fällt das ganze Ausgangsfoto heraus, nicht nur das eine Rechteck. Vorschläge für ein Motiv benutzen dieselben Vektoren und werden erst nach Bestätigung verknüpft.

---

# Text, Felder, Export

Die Textsuche ist kein Volltextindex. Ein Ausdruck wird mit `ILIKE` und Escape über Notiz, Tags, Feldwerte, Feldbezeichnung, Motivtitel und Vorgangstitel gelegt. Die Bildinhalt-Suche setzt diesen Textfilter aus, weil der Satz dort schon als CLIP-Vektor ankommt.

| Feldtyp | Gespeichert als | Filter |
| --- | --- | --- |
| `text`, `select` | `value_text` | Teilstring |
| `number`, `date`, `bool` | `value_text` | Vergleich je Typ |

Schlüssel und Typ eines Feldes bleiben fest, Bezeichnung und Optionen sind änderbar. Pflichtfelder prüft die API, sobald der Client eine Feldmap mitschickt. Ältere Clients ohne Map behalten das bisherige Verhalten.

Export und Auswertung nutzen dieselben Filter. CSV und JSON enthalten höchstens 10 000 Zeilen und keine Bild-URLs oder Object-IDs. Die Zusammenfassung zählt, gruppiert Tags, Motive und Felder und liefert Zeitverlauf und Hotspots.

---

# Sitzung, Passwort, Rollen

Lokale Passwörter liegen als bcrypt mit Default-Cost. Kürzer als acht Zeichen, gleich dem Benutzernamen oder eines der gesperrten Wörter (`admin`, `password`, `schmutzfink` und weitere) gilt als schwach. Das Seed-Passwort `admin` erzwingt einen Wechsel; bis dahin bleibt der Bestand zu.

| Rolle | Schreiben | Verwalten | Löschen ausführen |
| --- | --- | --- | --- |
| `admin` | ja | ja | nur mit Zusatzflag |
| `user`, Sachbearbeiter | ja | nein | nur mit Zusatzflag |
| `searcher` | nein | nein | nein |

Die Sitzung ist ein Cookie `sf_session` über sieben Tage. Der Wert sind 32 Zufallsbytes als Hex; in `sessions` steht nur der SHA-256. Fünf Fehlversuche sperren IP und Benutzername für fünf Minuten, gezählt im Speicher des Prozesses. `COOKIE_SECURE=auto` setzt das Secure-Flag, sobald der Request als HTTPS ankommt.

---

# Active Directory nur als Kennwort

Steht `LDAP_URL`, prüft der API-Prozess das Kennwort per Simple-Bind, Timeout fünf Sekunden. Rolle, Sperre und Löschflag bleiben in Schmutzfink. Es gibt keine Gruppensynchronisation und keinen SSO-Redirect. Ein lokales Admin-Konto bleibt der Zugang, wenn der Domain Controller ausfällt.

| Einstellung | Bind-Identität |
| --- | --- |
| `LDAP_DOMAIN` mit Punkt | `name@domäne`, also UPN |
| `LDAP_DOMAIN` ohne Punkt | `NETBIOS\name` |
| `LDAP_BIND_TEMPLATE` | ersetzt die Domain-Regel |
| Name enthält schon `@` oder `\` | Vorlage bleibt unangewendet |

Bevorzugt wird `ldaps` auf 636. Alternativ Klartext auf 389 plus `LDAP_STARTTLS=1`. Das Zertifikat muss zum Host in der URL passen. `LDAP_INSECURE_SKIP_VERIFY` ist ein Testschalter.

AD-Konten entstehen in der Benutzerverwaltung ohne Passwort oder über `POST /api/provision/users`. Der Provision-Token ist ein anderes Geheimnis als die lesenden `sft_`-Token. Austritt heißt `disabled: true`, sonst bleibt das Konto bis zur Sperre in der Oberfläche gültig.

---

# Lesende Token und Protokoll

Ein API-Token ist `sft_` plus 32 Zufallsbytes. Gespeichert wird der SHA-256, dazu Name, die ersten zwölf Zeichen als Prefix und ein Ablauf von 30, 90 oder 365 Tagen. Höchstens zehn aktive Token je Benutzer. Der Klartext erscheint einmal bei der Anlage.

**Was der Token darf.** Suche, Karte, Export und Zusammenfassung. Keine Originaldateien, keine Vorschaubilder, keine schreibenden Routen, keine Token-Verwaltung. Passwortwechsel oder Deaktivieren widerruft alle Token des Kontos.

**Protokoll.** Anmeldungen, Imports, Änderungen und die automatische Standortentfernung schreiben einen Audit-Satz mit Benutzer und Gegenstand. Die Fristen-Läufe laufen unter dem Namen `system`.

Ein späteres lokales Sprachmodell darf nur über diese lesende API formulieren. Im Go-Prozess gibt es keinen Chat-Client und keinen Bildversand an Dritte.

---

# Standort befristen, Foto behalten

Ein Vorgang ist zivilrechtlich oder strafrechtlich. Die Frist von drei beziehungsweise fünf Jahren beginnt mit dem Schließen, gerechnet als `closed_at` plus Intervall. Ohne Vorgang läuft keine automatische Standortfrist. Der Worker prüft beim Start und danach stündlich.

| Nach Fristablauf | Bleibt | Fällt weg |
| --- | --- | --- |
| Standortschwärzung | Foto, Sichtungen, Motiv, Notiz, Tags | GPS, Adresse inklusive PLZ |
| Vollständige Löschung | Antrag durch Sachbearbeiter | Ausführung nur durch Löschbeauftragte |

Nach der Schwärzung nimmt die API keine neuen Koordinaten für dieses Foto an. Wer einen Löschantrag gestellt hat, darf ihn nicht selbst ausführen. Sucher können das Löschflag nicht nutzen.

---

# Fundorte, Adresse, öffentliche Vorschau

Die Web-Karte ist Leaflet mit OpenStreetMap-Kacheln. Marker gibt es nur für Fotos mit gesetzten Koordinaten. Dieselben Filter wie in der Liste gelten auf der Karte, einschließlich der CLIP-Treffer.

**Adresse.** Geokodierung läuft serverseitig gegen Nominatim, eine Anfrage pro Sekunde, Timeout fünf Sekunden, vorwärts und rückwärts. Der Browser spricht OpenStreetMap dafür nicht direkt an.

**Öffentliche Vorschau.** Ein Admin kann die Karte ohne Anmeldung freigeben. Sichtbar sind Fundorte und die kleine Vorschau. Notizen, Originale, Filter, Export und die Detailseite bleiben zu.

In der Nähe-Suche der App akzeptiert die API einen Radius von 10 bis 200 Metern und höchstens 80 Treffer. Die App wählt 50 bis 150 Meter je nach GPS-Genauigkeit.

---

# Flutter nur für die Aufnahme

Die App unter `mobile/` ist Flutter, Android zuerst, iOS vorbereitet. Sie spricht dieselbe Cookie-Sitzung `sf_session`. API-Token bleiben lesend und sind dort kein Upload-Weg. Eine signierte Release-APK liegt im Portal unter Konto, Android-App.

| Funktion | Technik |
| --- | --- |
| Aufnahme | `image_picker`, Geräte-Kamera |
| Ort | `geolocator` zum Auslösezeitpunkt, Genauigkeit in Metern |
| Schlechter Fix | Pin auf `flutter_map`, sonst später im Web |
| Offline | Queue auf dem Gerät, Drain wenn `connectivity_plus` Netz sieht |
| Serie | Notiz, Tags, Vorgang, Motiv und Pflichtfelder gelten für alle |
| Vor dem Auslösen | Funde in der Nähe, Motiv und offenen Vorgang übernehmen |
| Kontrollfoto | Letztes Foto eines Motivs 90 Tage oder älter |
| Ausschnitt | Grobes Rechteck nach einer Einzelaufnahme |
| Weiterarbeit | Deep-Link auf `/datensatz/{id}` im Web |

---

# Pakete und feste Grenzen

Auslieferungsbuilds entstehen auf einem Windows-Rechner mit Go und Node. `pack.ps1` baut die Oberfläche, legt Modell, ONNX Runtime und Tokenizer bei und schreibt ein ZIP. Das Ziel braucht dafür kein Internet. `build-linux.ps1` tauscht nur das Binary; die Modelle bleiben auf dem Server unter `vendor/`.

| Paket | Ziel |
| --- | --- |
| `win-native-pg` | Windows-Dienst, PostgreSQL 16 mit pgvector |
| `win-pg-container` | Windows, Postgres als Container |
| `linux-vm` | systemd, natives PostgreSQL, `install.sh` |
| `linux-compose` | App und Postgres als Compose-Stack |

nginx nimmt 80 und 443 an und reicht an `127.0.0.1:8787` weiter. `client_max_body_size` muss mindestens 110 MB sein, wegen ZIP-Import. `TRUST_PROXY=auto` liest die Client-Adresse nur hinter diesem Proxy.

Festgehaltene Grenzen: ein Mandant, Inferenz im API-Prozess, Motive nur nach Bestätigung, Anmeldung lokal oder per LDAP-Bind, Aufnehmen in Flutter und Auswerten im Web. Ein Detektor für automatische Rechtecke und quantisierte CLIP-Modelle sind vorgemerkt, noch nicht gebaut.
