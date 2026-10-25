import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api.ts'
import { formatWhen } from '../format.ts'
import { useAuth } from '../auth.tsx'
import { canWriteCatalog, motifPath, type Motif } from '../types.ts'
import { useTileSize } from './TileSizeControl.tsx'

export function MotifsPage() {
  const { user } = useAuth()
  const [items, setItems] = useState<Motif[]>([])
  const [error, setError] = useState('')
  const [tileSize] = useTileSize()

  useEffect(() => {
    api
      .motifs()
      .then((res) => {
        setItems(res.items)
        setError('')
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }, [])

  const total = items.length
  const countLabel = total === 1 ? '1 Motiv' : `${total} Motive`

  return (
    <section>
      <div className="page-head">
        <h1>Motive</h1>
        <p>
          {countLabel}. Sichtungen werden nur per Hand zusammengelegt — nichts passiert automatisch.
        </p>
      </div>
      {error ? <p className="error">{error}</p> : null}
      {items.length === 0 && !error ? (
        <p className="empty">
          {canWriteCatalog(user)
            ? 'Noch keine Motive. Legen Sie eines an einer Sichtung an.'
            : 'Noch keine Motive.'}
        </p>
      ) : null}
      <ul className="record-list" data-tile-size={tileSize}>
        {items.map((m) => (
          <li key={m.id}>
            <Link to={motifPath(m.id)} className="record-row">
              {m.thumb_url ? <img src={m.thumb_url} alt="" /> : <div className="thumb-placeholder" />}
              <div>
                <strong>{m.title || 'Motiv'}</strong>
                <p>
                  {m.sighting_count === 1 ? '1 Sichtung' : `${m.sighting_count} Sichtungen`}
                  {m.first_at || m.last_at
                    ? ` · ${formatWhen(m.first_at)} – ${formatWhen(m.last_at)}`
                    : ''}
                </p>
                {m.note ? <p className="muted">{m.note}</p> : null}
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
