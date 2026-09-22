import { useState } from 'react'
import { api } from '../api.ts'
import type { Filters } from '../types.ts'

type Props = {
  filters: Filters
  gpsOnly?: boolean
}

export function ExportButtons({ filters, gpsOnly }: Props) {
  const [busy, setBusy] = useState<'csv' | 'json' | ''>('')
  const [error, setError] = useState('')
  const [hint, setHint] = useState('')

  async function download(format: 'csv' | 'json') {
    setBusy(format)
    setError('')
    setHint('')
    try {
      const next = gpsOnly ? { ...filters, hasGps: 'true' as const } : filters
      const result = await api.exportRecords(next, format)
      if (result.truncated) {
        setHint(`Nur die ersten ${result.count} von ${result.total} Treffern.`)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Export fehlgeschlagen')
    } finally {
      setBusy('')
    }
  }

  return (
    <div className="export-actions">
      <span className="muted">Treffer exportieren</span>
      <div className="export-buttons">
        <button type="button" className="secondary" disabled={!!busy} onClick={() => void download('csv')}>
          {busy === 'csv' ? 'CSV …' : 'CSV'}
        </button>
        <button type="button" className="secondary" disabled={!!busy} onClick={() => void download('json')}>
          {busy === 'json' ? 'JSON …' : 'JSON'}
        </button>
      </div>
      {error ? <p className="error">{error}</p> : null}
      {hint ? <p className="muted">{hint}</p> : null}
    </div>
  )
}
