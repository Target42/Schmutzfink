import { useEffect, useState } from 'react'
import { api } from '../api.ts'
import type { CustomField } from '../types.ts'

export function useCustomFields() {
  const [items, setItems] = useState<CustomField[]>([])
  useEffect(() => {
    api
      .fields()
      .then((res) => setItems(res.items))
      .catch(() => setItems([]))
  }, [])
  return items
}

export function FieldInputs({
  defs,
  values,
  onChange,
}: {
  defs: CustomField[]
  values: Record<string, string>
  onChange: (next: Record<string, string>) => void
}) {
  if (!defs.length) return null
  function set(key: string, value: string) {
    onChange({ ...values, [key]: value })
  }
  return (
    <fieldset className="custom-fields">
      <legend>Eigene Felder</legend>
      {defs.map((d) => (
        <label key={d.id}>
          {d.label}
          {d.required ? ' *' : ''}
          {d.type === 'select' ? (
            <select value={values[d.key] ?? ''} onChange={(e) => set(d.key, e.target.value)}>
              <option value="">{d.required ? 'Bitte wählen' : '—'}</option>
              {d.options.map((opt) => (
                <option key={opt} value={opt}>
                  {opt}
                </option>
              ))}
            </select>
          ) : d.type === 'bool' ? (
            <select value={values[d.key] ?? ''} onChange={(e) => set(d.key, e.target.value)}>
              <option value="">—</option>
              <option value="ja">ja</option>
              <option value="nein">nein</option>
            </select>
          ) : (
            <input
              type={d.type === 'number' ? 'number' : d.type === 'date' ? 'date' : 'text'}
              value={values[d.key] ?? ''}
              required={d.required}
              onChange={(e) => set(d.key, e.target.value)}
            />
          )}
        </label>
      ))}
    </fieldset>
  )
}

export function formatFieldValues(defs: CustomField[], values?: Record<string, string>) {
  if (!values) return []
  return defs
    .filter((d) => values[d.key])
    .map((d) => `${d.label}: ${values[d.key]}`)
}
