import { type FormEvent, useEffect, useState } from 'react'
import { api } from '../api.ts'
import type { CustomField, CustomFieldType } from '../types.ts'

const typeLabels: Record<CustomFieldType, string> = {
  text: 'Text',
  number: 'Zahl',
  date: 'Datum',
  bool: 'Ja/Nein',
  select: 'Auswahl',
}

function parseOptions(raw: string) {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

export function FieldsPage() {
  const [items, setItems] = useState<CustomField[]>([])
  const [editing, setEditing] = useState<CustomField | null>(null)
  const [label, setLabel] = useState('')
  const [type, setType] = useState<CustomFieldType>('text')
  const [options, setOptions] = useState('')
  const [required, setRequired] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)

  function reload() {
    api
      .fields()
      .then((res) => setItems(res.items))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }

  useEffect(() => {
    reload()
  }, [])

  function startEdit(field: CustomField) {
    setEditing(field)
    setLabel(field.label)
    setType(field.type)
    setOptions(field.options.join(', '))
    setRequired(field.required)
    setError('')
    setNotice('')
  }

  function resetForm() {
    setEditing(null)
    setLabel('')
    setType('text')
    setOptions('')
    setRequired(false)
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setNotice('')
    setBusy(true)
    const parsed = parseOptions(options)
    try {
      if (editing) {
        await api.patchField(editing.id, {
          label,
          required,
          sort_order: editing.sort_order,
          options: editing.type === 'select' ? parsed : editing.options,
        })
        setNotice('Feld gespeichert.')
        resetForm()
      } else {
        await api.createField({
          label,
          type,
          required,
          options: parsed,
        })
        setNotice('Feld angelegt.')
        resetForm()
      }
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Speichern fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function remove(field: CustomField) {
    if (!window.confirm(`Feld „${field.label}“ löschen? Alle gespeicherten Werte gehen verloren.`)) return
    setError('')
    try {
      await api.deleteField(field.id)
      if (editing?.id === field.id) resetForm()
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Löschen fehlgeschlagen')
    }
  }

  return (
    <section className="narrow">
      <div className="page-head">
        <h1>Felder</h1>
        <p>
          Nur Admins legen Felder an und ändern Bezeichnung, Optionen oder Pflicht. Typ und Schlüssel
          bleiben fest. Sachbearbeiter füllen die Werte an der Sichtung. Die Textsuche findet sie mit.
        </p>
      </div>
      {error ? <p className="error">{error}</p> : null}
      {notice ? <p className="ok">{notice}</p> : null}
      <ul className="plain-list">
        {items.map((f) => (
          <li key={f.id}>
            <strong>{f.label}</strong> · {typeLabels[f.type]} · <code>{f.key}</code>
            {f.required ? ' · Pflicht' : ''}
            {f.options.length ? ` · ${f.options.join(', ')}` : ''}
            {' '}
            <button type="button" className="linkish" onClick={() => startEdit(f)}>
              ändern
            </button>
            {' '}
            <button type="button" className="linkish" onClick={() => void remove(f)}>
              löschen
            </button>
          </li>
        ))}
      </ul>
      {items.length === 0 ? <p className="muted">Noch keine Felder.</p> : null}
      <form className="panel stack" onSubmit={(e) => void onSubmit(e)}>
        <h2>{editing ? 'Feld ändern' : 'Neu anlegen'}</h2>
        <label>
          Bezeichnung
          <input value={label} onChange={(e) => setLabel(e.target.value)} required />
        </label>
        {editing ? (
          <p className="muted">
            Typ {typeLabels[editing.type]}, Schlüssel <code>{editing.key}</code> — fest nach dem Anlegen.
          </p>
        ) : (
          <label>
            Typ
            <select value={type} onChange={(e) => setType(e.target.value as CustomFieldType)}>
              {Object.entries(typeLabels).map(([k, name]) => (
                <option key={k} value={k}>
                  {name}
                </option>
              ))}
            </select>
          </label>
        )}
        {(editing ? editing.type : type) === 'select' ? (
          <label>
            Optionen
            <input
              value={options}
              onChange={(e) => setOptions(e.target.value)}
              placeholder="offen, in Arbeit, erledigt"
              required
            />
          </label>
        ) : null}
        <label className="check-row">
          <input type="checkbox" checked={required} onChange={(e) => setRequired(e.target.checked)} />
          Pflichtfeld
        </label>
        <div className="user-actions">
          <button type="submit" disabled={busy}>
            {editing ? 'Speichern' : 'Anlegen'}
          </button>
          {editing ? (
            <button type="button" className="secondary" onClick={resetForm}>
              Abbrechen
            </button>
          ) : null}
        </div>
      </form>
    </section>
  )
}
