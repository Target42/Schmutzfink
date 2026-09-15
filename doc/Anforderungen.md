# Schmutzfink

Inventar von Graffiti-Fotos mit Ort, Zeit und Metadaten. Später nutzbar für Gemeinden, Polizei u. ä.

## Idee

Es soll eine Datenbank aufgebaut werden, die Bilder (mit GPS-Koordinaten, Zeitpunkt usw.) von Graffiti enthält.

Bedienung über Clients, die nur mit dem Backend sprechen. Zuerst eine Web-GUI. Später möglich: Desktop (z. B. Qt) und Android (Direktaufnahme ins System).

Funktionen im Zielbild:

- Bilder hochladen und speichern
- ähnliche Bilder per KI finden
- textuelle Suche (Notizen, Tags, benutzerdefinierte Felder)
- Karte mit den Treffern
- benutzerdefinierte Felder (Behörden-Kontext)
- Anmeldung (intern und optional AD per LDAP-Bind)

## Architektur (festgelegt)

- **Ein Backend.** Alle Clients greifen nur darauf zu (HTTP-API). Keine eigene Business-Logik in den Clients.
- **Backend in Go.** Kein Python-Runtime im Betrieb. KI-Ähnlichkeit als Go + ONNX Runtime, nicht als Python-Dienst.
- **Entwicklung unter Windows.** Linux nur als Laufzeit (Docker / ggf. WSL2), nicht als Arbeitsplatzwechsel.
- **Dateien hinter einem Storage-Interface.** Lokales Dateisystem oder S3/MinIO, gewählt über `.env` (`STORAGE_BACKEND`, `STORAGE_DIR` bzw. `S3_*`). In der Datenbank nur Object-IDs, keine physischen Pfade als Fachschlüssel. Keys: Jahr/Monat plus Hex-Shard.
- **Fachdaten in Postgres.** Bilder nicht als BLOBs in der Fachdatenbank. Vektoren in pgvector.

## Fachobjekte

- **Foto:** Datei, EXIF, GPS, Aufnahmezeit, Upload. Einmal speichern, Träger für alles darauf.
- **Sichtung:** ein Motiv auf einem Foto — Rechteck (oder ganzes Bild), eigenes Embedding, eigene Notiz/Tags. Liste, Karte und Ähnlichkeitssuche arbeiten auf Sichtungen.
- **Motiv:** mehrere Sichtungen über Fotos hinweg (anderer Winkel, anderer Tag, Übermalung). Erst zuordnen, nicht automatisch clustern.

Ein Foto ohne gezogene Rechtecke hat genau eine Sichtung „ganzes Bild“.

## Basis (v1)

Ziel: Ingest, Bestand, Karte, Anmeldung. Keine KI-Suche.

### Funktional

- Interne Benutzerverwaltung (Login nötig). Ein Mandant reicht.
- Bild hochladen; Original speichern; Thumbnails ablegen.
- EXIF auslesen: GPS (falls vorhanden), Aufnahmezeit. Fehlt GPS oder Zeit, bleibt der Datensatz gültig (Felder leer / manuell nachziehbar).
- Metadaten am Datensatz: Notiz/Beschreibung, optionale Tags, Aufnahmeort (Koordinaten), Aufnahmezeit, Upload-Zeit, wer hochgeladen hat.
- Liste/Detailansicht der Datensätze.
- Karte mit Markern für Datensätze, die Koordinaten haben.
- Filter auf der Karte und in der Liste nach Zeitraum, Umkreis und vorhandenem GPS.
- Einfache Volltextsuche über gespeicherte Texte (Notiz, Tags, Motivtitel, Vorgänge, eigene Felder).
- Admin-definierte Felder an Sichtungen (Text, Zahl, Datum, Ja/Nein, Auswahl), inkl. Suche und Filter.

### Technisch

- Go-API + Web-UI (eine Oberfläche, Desktop und Handy im Browser).
- Postgres für Metadaten und Benutzer.
- Lokales Dateisystem oder S3/MinIO für Originale und Thumbnails, hinter dem Storage-Interface.
- Docker für abhängige Dienste (mindestens Postgres).
- Datensätze mit `embedding_status = pending` angelegt, damit ein späterer Worker den Bestand nachziehen kann.

## Suche (v2)

Ziel: ähnliche Bilder und semantische Textsuche über den Bildinhalt. Weiterhin kein Python im Betrieb.

### Funktional

- Nach dem Upload wird asynchron ein CLIP-Embedding berechnet (nicht im HTTP-Request).
- Bestand ohne Embedding wird nachgezogen (Backfill über `embedding_status`).
- Am Datensatz: „ähnliche Bilder“ (Bild-zu-Bild, Cosinus).
- In Liste und Karte: Suchmodus **Bildinhalt** — Freitext wird mit dem CLIP-Textencoder in denselben Vektorraum gelegt.
- Bestehende Filter (Zeitraum, GPS, Umkreis) gelten auch für die KI-Treffer.
- Ohne geladenes Modell bleibt v1 nutzbar; die Bildsuche meldet dann klar, dass sie nicht bereit ist.
- Optional: Erkennungsrechteck am Foto (vom Bearbeiter aufgezogen). CLIP nutzt nur diesen Ausschnitt; das Original bleibt vollständig.

### Technisch

- OpenCLIP ViT-B/32 als ONNX (Bild- und Textencoder), geladen über ONNX Runtime ohne CGO (`pure-onnx` / `purego`).
- Runtime-DLL und Modelldateien beim ersten Start laden/cachen, nicht ins Git.
- Embeddings als `vector(512)` in Postgres (pgvector), Index HNSW mit Cosinus.
- Ein Worker im API-Prozess: `pending` → `processing` → `ready` / `failed`.
- Eine ONNX-Session wiederverwenden, nicht pro Request neu laden.

## Sichtungen (v3)

Ziel: ein Foto kann mehrere Graffiti tragen. Die Suche trifft Motive, nicht nur Dateien.

### Funktional

- Am Foto beliebig viele Rechtecke (Sichtungen). Jede Sichtung: Ausschnitt, Notiz, Tags, eigenes CLIP-Embedding.
- Ohne Rechteck bleibt eine Sichtung über das ganze Bild (heutiges Verhalten).
- Liste, Karte, Bildinhalt-Suche und „ähnliche Bilder“ beziehen sich auf Sichtungen. Treffer zeigen das Foto plus markierten Ausschnitt.
- GPS und Zeitpunkt kommen vom Foto, nicht von der Sichtung.
- Rechteck ändern oder löschen löst Neu-Embedding nur dieser Sichtung aus.

### Technisch

- Foto-Datensatz und Sichtung trennen (eigene Tabelle). Embedding hängt an der Sichtung.
- Bestehende Datensätze: eine Sichtung „ganzes Bild“ bzw. das bisherige einzelne ROI übernehmen (kein Datenverlust).
- Worker wie in v2, Job-Quelle ist die Sichtung.

## Motive (v4)

Ziel: dieselbe Arbeit über mehrere Fotos zusammenhalten.

### Funktional

- Sichtungen manuell einem Motiv zuordnen (und wieder lösen).
- Motiv-Ansicht: alle Sichtungen, Zeitreihe, Karte der Fundorte.
- Von einer Sichtung: „ähnliche Sichtung diesem Motiv zuordnen“ (Vorschlag aus CLIP, Bestätigung durch den Nutzer).
- Kein automatisches Zusammenlegen ohne Bestätigung.

### Technisch

- Motiv-Tabelle, Sichtung optional `motif_id`.
- Ähnlichkeitsvorschläge nutzen die bestehenden Vektoren, keine Extra-Modelle.

## Später (nicht v3/v4)

- Automatische Rechtecke per Detektor — erst mit großem, gepflegtem Bestand und Erfahrung aus manuellen Sichtungen.
- Native Android-App (Kamera + GPS direkt).
- Qt-/Desktop-Client.
- Quantisierte CLIP-Varianten (INT8) nur nach Messung.

## Vorgänge und Löschfristen

Ziel: Standort nicht länger als nötig halten, das Foto als Nachweis behalten.

### Funktional

- Sachbearbeiter legen **Vorgänge** an (Titel/Aktenzeichen, Notiz) und ordnen Fotos zu.
- Jeder Vorgang ist **zivilrechtlich (3 Jahre)** oder **strafrechtlich (5 Jahre)**. Die Frist beginnt erst, wenn ein Bearbeiter den Vorgang schließt.
- Ist die Frist erreicht, werden GPS-Koordinaten und die gespeicherte Adresse inklusive PLZ entfernt. Foto, Sichtungen, Motive, Notizen und Tags bleiben.
- Ohne Vorgang läuft keine automatische Standortfrist.
- Sachbearbeiter können weiterhin die vollständige Löschung beantragen; nur Löschbeauftragte führen sie aus.

### Technisch

- Vorgang-Tabelle, Foto optional `case_id`. Worker im API-Prozess prüft stündlich fällige Standorte.
- Nach der Standortentfernung lässt sich GPS nicht wieder setzen.

## Auswertungen und Maschinen-Zugang

Ziel: weitere Auswertungen (Hotspots, Serien, Zeitverläufe, eigene Felder) über denselben Backend-Weg wie die Web-UI. Bildähnlichkeit bleibt CLIP lokal. Sprachmodelle sind optionaler Client, nicht Teil des Servers.

### Funktional

- Maschinen-Zugang neben der Cookie-Sitzung: API-Token (`Authorization: Bearer sft_…`), nur Lesen (Suche, Karte, Export, Metadaten). Keine Originale/Thumbnails, keine Schreib- oder Admin-Routen, keine Token-Verwaltung über Token. Nur Hash in der Datenbank, Name, Prefix, Ablauf (30/90/365 Tage), Widerruf, Audit. Höchstens 10 aktive Token je Benutzer. Passwortändern oder Deaktivieren widerruft alle Token.
- Export der aktuellen Treffermenge (JSON/CSV) mit denselben Filtern wie Liste und Karte — in der Web-UI und als `GET`/`POST /api/records/export`. Ohne Bild-URLs und Object-IDs; eigene Felder als Spalten bzw. Objekt. Höchstens 10 000 Zeilen, danach `truncated`.
- Verdichtete Zusammenfassungen statt Rohbilder: Zählungen, Motiv-Titel, Tags, Koordinaten/Hotspots, Zeitverlauf, eigene Felder. Web-UI **Auswertungen** und `GET`/`POST /api/records/summary` (gleiche Filter, keine Bilder).
- Ein späteres LLM (lokal) darf nur über diese API lesen und formulieren; die Wahrheit bleibt in Postgres und CLIP.

### Technisch

- Kein OpenAI/Ollama/Chat im Go-Prozess.
- Kein Extra-Gateway. Token und Export hängen an den bestehenden Listen-/Filter-Endpunkten.
- Token widerrufbar, auditiert, nicht im Klartext speichern.

## Nicht-Ziele

- Python-Serving oder PyTorch im Betrieb.
- Cloud-LLM oder Bildversand an Dritte für Analyse.
- Mehrere parallele UIs (Web + Qt + Android).
- Echtzeit-Inferenz beim Upload.
- Öffentliche Registrierung / Mehrbenutzer-Produktivbetrieb für Dritte.
- Mehrere Mandanten — ein Mandant reicht.
- Entra / OIDC / SSO-Redirect — Anmeldung bleibt lokal oder AD per LDAP-Bind.
