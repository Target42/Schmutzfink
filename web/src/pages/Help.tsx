import { Link } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { canExecuteDeletion, canWriteCatalog, isAdmin, roleLabel } from '../types.ts'

const toc = [
  { id: 'begriffe', label: 'Begriffe' },
  { id: 'anmeldung', label: 'Anmeldung und Konto' },
  { id: 'rollen', label: 'Rollen' },
  { id: 'bestand', label: 'Bestand' },
  { id: 'suche', label: 'Suche und Filter' },
  { id: 'karte', label: 'Karte' },
  { id: 'auswertungen', label: 'Auswertungen' },
  { id: 'hochladen', label: 'Hochladen' },
  { id: 'datensatz', label: 'Foto und Sichtungen' },
  { id: 'motive', label: 'Motive' },
  { id: 'vorgaenge', label: 'Vorgänge' },
  { id: 'loeschungen', label: 'Löschungen' },
  { id: 'felder', label: 'Eigene Felder' },
  { id: 'benutzer', label: 'Benutzer' },
  { id: 'protokoll', label: 'Protokoll' },
  { id: 'token', label: 'API-Token' },
  { id: 'grenzen', label: 'Grenzen und Hinweise' },
] as const

export function HelpPage() {
  const { user } = useAuth()
  const writer = canWriteCatalog(user)
  const admin = isAdmin(user)
  const officer = canExecuteDeletion(user)

  return (
    <article className="help-page">
      <div className="page-head">
        <h1>Hilfe</h1>
        <p>
          Schmutzfink ist ein Inventar für Graffiti-Fotos: Ort, Zeit, Notizen und ähnliche
          Motive. Sie sind angemeldet als <strong>{user?.username}</strong>
          {' '}({roleLabel(user?.role ?? 'user')}
          {officer ? ', Löschbeauftragter' : ''}).
        </p>
      </div>

      <nav className="help-toc panel" aria-label="Inhalt">
        <p className="eyebrow">Inhalt</p>
        <ol>
          {toc.map((item) => (
            <li key={item.id}>
              <a href={`#${item.id}`}>{item.label}</a>
            </li>
          ))}
        </ol>
      </nav>

      <section id="begriffe" className="help-section">
        <h2>Begriffe</h2>
        <dl className="help-dl">
          <div>
            <dt>Foto</dt>
            <dd>
              Die hochgeladene Datei. GPS, Aufnahmezeit und der Löschantrag hängen am Foto, nicht
              an einzelnen Ausschnitten.
            </dd>
          </div>
          <div>
            <dt>Sichtung</dt>
            <dd>
              Ein Motiv auf einem Foto — entweder das ganze Bild oder ein aufgezogenes Rechteck.
              Notiz, Tags, eigene Felder und die Bildähnlichkeit gehören zur Sichtung. Liste, Karte
              und Suche arbeiten auf Sichtungen.
            </dd>
          </div>
          <div>
            <dt>Motiv</dt>
            <dd>
              Mehrere Sichtungen derselben Arbeit über verschiedene Fotos (anderer Winkel, anderer
              Tag, Übermalung). Zusammenlegen geschieht nur von Hand, nie automatisch.
            </dd>
          </div>
          <div>
            <dt>Vorgang</dt>
            <dd>
              Eine Akte, der Fotos zugeordnet werden: zivilrechtlich (3 Jahre) oder strafrechtlich
              (5 Jahre). Die Frist beginnt erst, wenn ein Bearbeiter den Vorgang schließt. Nach
              Ablauf fallen GPS und Adresse bis zur PLZ weg. Das Foto bleibt.
            </dd>
          </div>
        </dl>
        <p>
          Ein Foto ohne Rechteck hat genau eine Sichtung „ganzes Bild“. Mehrere Graffiti auf einem
          Foto legen Sie als weitere Sichtungen an.
        </p>
      </section>

      <section id="anmeldung" className="help-section">
        <h2>Anmeldung und Konto</h2>
        <p>
          Der Bestand ist nicht öffentlich. Nach der Anmeldung bleibt die Sitzung über ein Cookie
          erhalten, bis Sie sich oben rechts abmelden.
        </p>
        <ul>
          <li>
            Ein vom Admin gesetztes Startpasswort müssen Sie beim ersten Login ändern. Bis dahin
            bleiben Fotos und Metadaten gesperrt.
          </li>
          <li>
            Ein neues Passwort braucht mindestens 8 Zeichen, darf nicht der Benutzername sein und
            kein offensichtliches Standardwort (<code>admin</code>, <code>passwort</code>, …).
          </li>
          <li>
            Fünf fehlgeschlagene Versuche sperren Anmeldung für diese Adresse bzw. diesen Namen
            für fünf Minuten.
          </li>
          <li>
            Unter Ihrem Namen oben rechts ändern Sie das Passwort (lokale Konten) und öffnen{' '}
            <Link to="/token">API-Token</Link>. Ein Passwortwechsel widerruft alle eigenen Token.
            AD-Konten ändern das Kennwort nur im Verzeichnis.
          </li>
        </ul>
      </section>

      <section id="rollen" className="help-section">
        <h2>Rollen</h2>
        <p>Jeder sieht Bestand, Karte, Motive und Vorgänge. Schreiben und Verwalten hängen an der Rolle.</p>
        <dl className="help-dl">
          <div>
            <dt>Sucher</dt>
            <dd>
              Nur suchen — auch mit einem Vergleichsbild, das nicht im Bestand landet. Kein
              Upload, keine Bearbeitung, keine Motive oder Vorgänge anlegen, keine Löschanträge.
            </dd>
          </div>
          <div>
            <dt>Sachbearbeiter</dt>
            <dd>
              Bestand pflegen: hochladen, Sichtungen, Motive und Vorgänge bearbeiten, Löschung
              beantragen.
            </dd>
          </div>
          <div>
            <dt>Admin</dt>
            <dd>
              Zusätzlich Benutzer, eigene Felder und das Protokoll. Admins können den Bestand
              ebenfalls pflegen.
            </dd>
          </div>
          <div>
            <dt>Löschbeauftragter</dt>
            <dd>
              Zusatzrolle für Sachbearbeiter oder Admins, nicht für Sucher. Nur sie führen
              beantragte Löschungen aus — nicht den eigenen Antrag.
            </dd>
          </div>
        </dl>
      </section>

      <section id="bestand" className="help-section">
        <h2>Bestand</h2>
        <p>
          Unter <Link to="/">Bestand</Link> stehen die Sichtungen als Kacheln: Vorschau, Notiz,
          Aufnahme- oder Uploadzeit, Koordinaten, Tags, wer hochgeladen hat, zugeordnetes Motiv
          und eigene Felder. Bei einer Bildsuche erscheint zusätzlich die Ähnlichkeit in Prozent.
        </p>
        <ul>
          <li>Ein Klick öffnet das Foto mit der gewählten Sichtung.</li>
          <li>Viele Treffer werden seitenweise angezeigt (12 je Seite).</li>
          {writer ? (
            <li>
              Ohne Treffer: zuerst unter <Link to="/hochladen">Hochladen</Link> ein Foto anlegen.
            </li>
          ) : null}
        </ul>
      </section>

      <section id="suche" className="help-section">
        <h2>Suche und Filter</h2>
        <p>
          Dieselben Filter gelten in Liste, Karte und Auswertungen. Sie lassen sich kombinieren. Die Trennlinie
          zwischen Filtern und Treffern bzw. Karte lässt sich ziehen; ein Doppelklick setzt die
          Größe zurück.
        </p>
        <dl className="help-dl">
          <div>
            <dt>Text</dt>
            <dd>
              Findet Wörter in Notiz, Tags, Motivtitel, Vorgangstitel und eigenen Feldern.
              Geeignet für Namen, Straßen oder Aktenzeichen, die Sie selbst erfasst haben.
            </dd>
          </div>
          <div>
            <dt>Bildinhalt</dt>
            <dd>
              Freitext wird mit dem lokalen CLIP-Modell in denselben Raum gelegt wie die Fotos.
              Formulieren Sie, was zu sehen ist — etwa „rotes Tag an einer Wand“ oder „Schriftzug
              in Schwarz“. Optional ein Suchbild wählen: es dient nur dem Vergleich und wird nicht
              gespeichert. Die Mindestähnlichkeit filtert schwache Treffer; bei Freitext oft 30&nbsp;%
              wählen, beim Suchbild eher 50&nbsp;% oder strenger.
            </dd>
          </div>
          <div>
            <dt>Mindestähnlichkeit</dt>
            <dd>
              Steht in der Filterleiste (Liste und Karte) sowie bei ähnlichen Sichtungen.
              30, 50, 70 oder 85&nbsp;%. Wirkt auf Bildinhalt-Suche, ähnliche Sichtungen und
              Motiv-Vorschläge. Die Wahl bleibt im Browser gespeichert. Die Prozentzahl ist die
              CLIP-Ähnlichkeit, kein fachlicher Gleichheitswert.
            </dd>
          </div>
          <div>
            <dt>Zeitraum</dt>
            <dd>Filtert nach Aufnahmedatum (bzw. Upload, wenn keine Aufnahmezeit vorliegt).</dd>
          </div>
          <div>
            <dt>GPS</dt>
            <dd>Nur Sichtungen mit oder ohne Koordinaten. Auf der Karte entfällt dieser Filter.</dd>
          </div>
          <div>
            <dt>Motiv</dt>
            <dd>Ein bestimmtes Motiv oder nur Sichtungen ohne Motivzuordnung.</dd>
          </div>
          <div>
            <dt>Vorgang</dt>
            <dd>Ein bestimmter Vorgang oder nur Fotos ohne Vorgang.</dd>
          </div>
          <div>
            <dt>Eigene Felder</dt>
            <dd>
              Im Textmodus erscheint je definiertem Feld ein Filter (Auswahl, Ja/Nein, Datum, Zahl
              oder Freitext).
            </dd>
          </div>
          <div>
            <dt>Umkreis</dt>
            <dd>
              Ort suchen, auf der Karte einen Punkt setzen oder in der Liste „Auf Karte wählen“.
              Radius von 50&nbsp;m bis 10&nbsp;km. Danach „Umkreis aufheben“, um wieder den ganzen
              Bestand zu sehen.
            </dd>
          </div>
        </dl>
        <p>
          Oben rechts exportieren Sie die aktuelle Treffermenge als CSV oder JSON — ohne
          Bilddateien und ohne interne Datei-IDs. Höchstens 10&nbsp;000 Zeilen; darüber erscheint
          ein Hinweis, dass der Export gekürzt wurde. Auf der Karte enthält der Export nur Punkte
          mit Koordinaten.
        </p>
      </section>

      <section id="karte" className="help-section">
        <h2>Karte</h2>
        <p>
          <Link to="/karte">Karte</Link> zeigt alle Treffer mit GPS. Die Maus über einem Marker
          zeigt eine Bildvorschau. Ein Klick setzt den Umkreisfilter auf diesen Punkt und öffnet
          den Link zum Datensatz. Nach dem Upload springt die Karte auf die neue Sichtung.
        </p>
        <ul>
          <li>Auf die freie Karte klicken setzt den Mittelpunkt für den Umkreisfilter.</li>
          <li>Sichtungen ohne Koordinaten erscheinen hier nicht — sie stehen nur in der Liste.</li>
          <li>
            In der Detailansicht: blauer Pin = dieser Fundort, Fahnen = ähnliche Sichtungen mit
            GPS.
          </li>
        </ul>
      </section>

      <section id="auswertungen" className="help-section">
        <h2>Auswertungen</h2>
        <p>
          Unter <Link to="/auswertungen">Auswertungen</Link> gelten dieselben Filter wie in Liste
          und Karte. Statt einzelner Fotos sehen Sie Zählungen: Sichtungen und Fotos, mit/ohne GPS,
          Zeitverlauf nach Monat, häufigste Tags und Motive, eigene Felder und Hotspots (Raster
          rund 100&nbsp;m). Es werden keine Bilder ausgeliefert. API-Token dürfen denselben Endpunkt
          lesen: <code>GET /api/records/summary</code>.
        </p>
      </section>

      <section id="hochladen" className="help-section">
        <h2>Hochladen</h2>
        {writer ? (
          <>
            <p>
              Unter <Link to="/hochladen">Hochladen</Link> nehmen Sie Bilder oder ein ZIP entgegen.
              Zulässig: JPEG, PNG, WebP und GIF. Mehrere Fotos per Drag & Drop oder Mehrfachauswahl —
              Sie speichern nacheinander (GPS, Notiz, Tags je Bild). Foto, Formular und ZIP-Import
              lassen sich mit den Trennlinien in der Größe anpassen.
            </p>
            <h3>Bilder</h3>
            <ol>
              <li>
                Dateien wählen oder in die Ablagefläche ziehen. Steckt GPS im EXIF, wird der Ort
                übernommen. Bei mehreren Dateien erscheint eine Warteschlange.
              </li>
              <li>
                Fehlt der Ort: Adresse suchen, auf die Karte klicken oder Breite/Länge eintragen.
                Marker lassen sich ziehen. „Koordinaten entfernen“ löscht den Ort bewusst.
              </li>
              <li>
                Optional ein Rechteck aufziehen, wenn nur ein Ausschnitt das Motiv ist. CLIP
                rechnet dann nur mit diesem Bereich; das Original bleibt vollständig.
              </li>
              <li>
                Notiz, kommagetrennte Tags, eigene Felder und optional einen Vorgang ausfüllen,
                dann Speichern. Bei einer Warteschlange geht es mit dem nächsten Bild weiter;
                beim letzten öffnet sich die Karte.
              </li>
            </ol>
            <p>
              Den aktuellen Eintrag können Sie überspringen oder die ganze Auswahl leeren. Der
              gewählte Vorgang bleibt für die folgenden Bilder erhalten. Ist dieselbe Datei schon
              im Bestand (gleicher Inhaltshash), erscheint ein Link zum vorhandenen Foto statt
              eines Duplikats.
            </p>
            <h3>ZIP importieren</h3>
            <p>
              Archiv mit Bildern und einer <code>index.json</code> — wie unter{' '}
              <code>Beispiele/import</code> bzw. <code>Beispiele/raw</code>. Pro Eintrag: Dateiname,
              GPS, Adresse, Notiz. Bereits vorhandene Dateien werden übersprungen. Höchstens
              100&nbsp;MB und 500 Dateien. Nach dem Import sehen Sie Übernahmen, Duplikate und
              Hinweise.
            </p>
          </>
        ) : (
          <p className="help-note">
            Mit der Rolle Sucher können Sie keine Fotos hochladen. Bitte einen Sachbearbeiter oder
            Admin bitten.
          </p>
        )}
      </section>

      <section id="datensatz" className="help-section">
        <h2>Foto und Sichtungen</h2>
        <p>
          Die Detailseite zeigt das Original, alle Rechtecke und die Metadaten der gewählten
          Sichtung. „Original öffnen“ lädt die volle Datei. {writer ? '„Bearbeiten“ schaltet in den Änderungsmodus.' : ''}
          Die Trennlinien zwischen Foto, ähnlichen Treffern, Metadaten und Karte — und im
          Bearbeiten-Modus zwischen Foto und Formular — lassen sich ziehen; ein Doppelklick
          setzt die Größe zurück.
        </p>
        {writer ? (
          <>
            <h3>Bearbeiten</h3>
            <ul>
              <li>
                Sichtungen oben als Karten: Klick wählt eine aus. „Weitere Sichtung“ und dann ein
                Rechteck aufziehen legt eine neue an.
              </li>
              <li>
                Ein bestehendes Rechteck ändern Sie, indem Sie im Bearbeiten-Modus ein neues
                aufziehen. Das Embedding dieser Sichtung wird neu berechnet.
              </li>
              <li>
                „Diese Sichtung entfernen“ geht, wenn mehr als eine Sichtung da ist oder die
                gewählte ein Rechteck hat. Die letzte Ganzbild-Sichtung bleibt.
              </li>
              <li>
                Notiz, Tags und eigene Felder gelten für die gewählte Sichtung. Ort und
                Aufnahmezeit gelten für das ganze Foto.
              </li>
            </ul>
          </>
        ) : null}
        <h3>Ähnliche Sichtungen</h3>
        <p>
          Unter dem Foto erscheinen Treffer ab der gewählten Mindestähnlichkeit (Standard
          50&nbsp;%). Direkt nach dem Upload kann „Ähnlichkeit wird noch berechnet …“ stehen —
          die Berechnung läuft im Hintergrund. Schlägt sie fehl, bleibt der Datensatz trotzdem
          gültig.
        </p>
        {writer ? (
          <p>
            Gehört die aktuelle Sichtung zu einem Motiv, können Sie einen ähnlichen Treffer direkt
            „Diesem Motiv zuordnen“.
          </p>
        ) : null}
      </section>

      <section id="motive" className="help-section">
        <h2>Motive</h2>
        <p>
          <Link to="/motive">Motive</Link> listet alle manuell angelegten Gruppen mit Zeitraum und
          Anzahl der Sichtungen. Die Motivseite zeigt Karte, Zeitreihe und CLIP-Vorschläge ab der
          gewählten Mindestähnlichkeit. Die Bereiche lassen sich mit Trennlinien verschieben.
        </p>
        {writer ? (
          <ul>
            <li>
              An einer Sichtung „Motiv beginnen“ — Titel kommt zunächst aus der Notiz und lässt
              sich später ändern.
            </li>
            <li>Ähnliche Sichtungen einzeln zuordnen oder vom Motiv lösen.</li>
            <li>
              „Motiv auflösen“ entfernt nur die Gruppe. Die Fotos bleiben. Das letzte Lösen einer
              Sichtung löst das Motiv ebenfalls auf.
            </li>
          </ul>
        ) : (
          <p className="help-note">Sucher können Motive ansehen, aber nicht anlegen oder ändern.</p>
        )}
      </section>

      <section id="vorgaenge" className="help-section">
        <h2>Vorgänge</h2>
        <p>
          Unter <Link to="/vorgaenge">Vorgänge</Link> legen Sachbearbeiter Akten an und ordnen
          Fotos zu. Die Art bestimmt die Frist: zivilrechtlich drei Jahre, strafrechtlich fünf
          Jahre — gerechnet ab dem Tag, an dem ein Bearbeiter den Vorgang schließt. Karte,
          Fotoliste und Formular lassen sich mit Trennlinien verschieben.
        </p>
        <ul>
          <li>Ein Foto gehört höchstens zu einem Vorgang.</li>
          <li>Solange der Vorgang offen ist, läuft keine Standortfrist.</li>
          <li>
            Nach dem Schließen und Ablauf der Frist löscht das System GPS-Koordinaten und die
            gespeicherte Adresse inklusive PLZ. Das Bild, Notizen, Tags, Sichtungen und Motive
            bleiben.
          </li>
          <li>Vor der Standortentfernung lässt sich ein Vorgang wieder öffnen; die Frist stoppt.</li>
          <li>Ohne Vorgang läuft keine automatische Standortfrist.</li>
          <li>
            Ein vollständiges Löschen des Fotos bleibt der manuelle Antrag an den
            Löschbeauftragten.
          </li>
        </ul>
        {writer ? null : (
          <p className="help-note">Sucher können Vorgänge ansehen, aber nicht anlegen oder ändern.</p>
        )}
      </section>

      <section id="loeschungen" className="help-section">
        <h2>Löschungen</h2>
        <p>
          Fotos bleiben als möglicher Nachweis erhalten. Nach Ablauf der Vorgangsfrist fällt nur
          der genaue Standort weg. Sachbearbeiter können zusätzlich die vollständige Löschung
          beantragen (optionaler Grund). Das Foto bleibt sichtbar, bis ein Löschbeauftragter
          entscheidet.
        </p>
        <ul>
          <li>Der Antragsteller kann den eigenen Antrag zurückziehen.</li>
          <li>
            Löschbeauftragte lehnen ab oder löschen unwiderruflich — Foto, Sichtungen und Dateien.
          </li>
          <li>Eigene Anträge dürfen Sie nicht selbst ausführen, auch als Löschbeauftragter.</li>
          {officer ? (
            <li>
              Offene Anträge stehen unter <Link to="/loeschungen">Löschungen</Link> und am
              Datensatz.
            </li>
          ) : null}
        </ul>
      </section>

      <section id="felder" className="help-section">
        <h2>Eigene Felder</h2>
        {admin ? (
          <>
            <p>
              Unter <Link to="/felder">Felder</Link> definieren Sie Metadaten für den
              Behörden-Kontext: Text, Zahl, Datum, Ja/Nein oder Auswahl. Bezeichnung, Optionen und
              Pflicht lassen sich nachträglich ändern; Typ und Schlüssel nicht. Pflichtfelder müssen beim
              Speichern einer Sichtung gefüllt sein. Die Textsuche findet die Werte mit.
            </p>
            <p>
              Ein Feld löschen entfernt alle gespeicherten Werte. Sachbearbeiter füllen die Felder
              beim Upload und in der Bearbeitung.
            </p>
          </>
        ) : (
          <p>
            Admins legen die Felder fest. {writer ? 'Sie füllen sie an der Sichtung aus.' : 'Sie sehen die Werte in Liste und Detail.'}
          </p>
        )}
      </section>

      <section id="benutzer" className="help-section">
        <h2>Benutzer</h2>
        {admin ? (
          <>
            <p>
              Unter <Link to="/benutzer">Benutzer</Link> legen Sie Konten an, setzen Rolle und
              Löschbeauftragten, deaktivieren Konten und vergeben ein neues Startpasswort
              (mindestens 8 Zeichen). Die Person muss es beim nächsten Login ändern. AD-Konten
              brauchen kein Passwort in Schmutzfink — die Anmeldung prüft das Kennwort gegen das
              Verzeichnis. Der AD-Aufnahmeprozess kann dieselben Konten über{' '}
              <code>/api/provision/users</code> anlegen und deaktivieren.
            </p>
            <ul>
              <li>Der letzte aktive Admin kann nicht deaktiviert oder herabgestuft werden.</li>
              <li>Das eigene Konto verwalten Sie nicht in dieser Tabelle — Passwort oben rechts.</li>
              <li>Deaktivieren oder Passwort ändern widerruft die API-Token der Person.</li>
              <li>Bei AD-Konten gibt es kein Passwort setzen; deaktivieren sperrt den Zugang hier.</li>
            </ul>
          </>
        ) : (
          <p className="help-note">Nur Admins verwalten Konten. Neue Zugänge über Ihren Admin.</p>
        )}
      </section>

      <section id="protokoll" className="help-section">
        <h2>Protokoll</h2>
        {admin ? (
          <p>
            Unter <Link to="/protokoll">Protokoll</Link> stehen Anmeldungen, fehlgeschlagene
            Logins, Benutzeränderungen, Uploads, Imports, Sichtungen, Motive, Vorgänge, Felder,
            Standortfristen, Exporte und Token — mit Zeit, Konto, Objekt und IP.
          </p>
        ) : (
          <p className="help-note">Das Protokoll ist nur für Admins sichtbar.</p>
        )}
      </section>

      <section id="token" className="help-section">
        <h2>API-Token</h2>
        <p>
          Unter <Link to="/token">API-Token</Link> (Konto-Menü) erzeugen Sie einen lesenden
          Maschinen-Zugang. Header: <code>Authorization: Bearer sft_…</code>
        </p>
        <ul>
          <li>Erlaubt: Suche, Karte, Export, Auswertungen, Metadaten, Motive, Vorgänge.</li>
          <li>Nicht erlaubt: Originale, Thumbnails, Uploads, Admin, Token-Verwaltung.</li>
          <li>Höchstens 10 aktive Token je Konto. Laufzeit 30, 90 oder 365 Tage.</li>
          <li>
            Der volle Wert erscheint nur einmal. Danach sehen Sie nur das Prefix. Widerruf sofort.
          </li>
        </ul>
      </section>

      <section id="grenzen" className="help-section">
        <h2>Grenzen und Hinweise</h2>
        <ul>
          <li>
            Bildähnlichkeit läuft lokal (OpenCLIP). Es werden keine Originale an Dritte geschickt.
          </li>
          <li>
            Ohne geladenes Modell bleiben Liste, Karte und Upload nutzbar; Bildinhalt-Suche und
            „ähnliche Sichtungen“ melden dann, dass sie nicht bereit sind.
          </li>
          <li>
            Export und Auswertungen fassen höchstens 10&nbsp;000 Treffer. Darüber erscheint ein
            Hinweis, dass die Menge gekürzt wurde.
          </li>
          <li>
            Nach der Frist werden GPS und Adresse im Katalog entfernt. In der Originaldatei kann
            EXIF-GPS noch stecken — das Original wird nicht umgeschrieben.
          </li>
          <li>
            Die Kartenortssuche braucht eine Netzverbindung zur Geocodierung. Der Bestand selbst
            liegt auf diesem Server.
          </li>
        </ul>
        <p className="muted">
          Diese Hilfe beschreibt die Web-Oberfläche. Technische Hinweise für den Betrieb stehen in
          der Projektdokumentation neben der Anwendung.
        </p>
      </section>
    </article>
  )
}
