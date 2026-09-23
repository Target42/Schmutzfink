import { useRef, useState, type CSSProperties, type PointerEvent } from 'react'
import type { Roi } from '../types.ts'

export type RoiBox = {
  id: string
  roi: Roi | null
}

type Props = {
  src: string
  alt?: string
  value?: Roi | null
  onChange?: (roi: Roi | null) => void
  boxes?: RoiBox[]
  selectedId?: string | null
  onSelect?: (id: string) => void
  onDrawn?: (roi: Roi) => void
  adding?: boolean
  hint?: string
  readOnly?: boolean
}

type Pt = { x: number; y: number }

function clamp01(n: number) {
  return Math.min(1, Math.max(0, n))
}

function toLocal(el: HTMLElement, e: PointerEvent<HTMLElement>): Pt {
  const r = el.getBoundingClientRect()
  const w = r.width || 1
  const h = r.height || 1
  return {
    x: clamp01((e.clientX - r.left) / w),
    y: clamp01((e.clientY - r.top) / h),
  }
}

function fromCorners(a: Pt, b: Pt): Roi {
  const x = Math.min(a.x, b.x)
  const y = Math.min(a.y, b.y)
  return { x, y, w: Math.abs(b.x - a.x), h: Math.abs(b.y - a.y) }
}

function boxStyle(roi: Roi): CSSProperties {
  return {
    left: `${roi.x * 100}%`,
    top: `${roi.y * 100}%`,
    width: `${roi.w * 100}%`,
    height: `${roi.h * 100}%`,
  }
}

function RoiCanvas({
  src,
  boxes,
  selectedId,
  onSelect,
  onDrawn,
  readOnly = false,
  alt = '',
}: {
  src: string
  boxes: RoiBox[]
  selectedId?: string | null
  onSelect?: (id: string) => void
  onDrawn: (roi: Roi) => void
  readOnly?: boolean
  alt?: string
}) {
  const layerRef = useRef<HTMLDivElement>(null)
  const startRef = useRef<Pt | null>(null)
  const [draft, setDraft] = useState<Roi | null>(null)

  function onPointerDown(e: PointerEvent<HTMLDivElement>) {
    if (e.button !== 0) return
    const target = e.target as HTMLElement
    const boxId = target.closest('[data-box-id]')?.getAttribute('data-box-id')
    if (boxId) {
      onSelect?.(boxId)
      return
    }
    if (readOnly) return
    const el = layerRef.current
    if (!el) return
    el.setPointerCapture(e.pointerId)
    startRef.current = toLocal(el, e)
    setDraft({ x: startRef.current.x, y: startRef.current.y, w: 0, h: 0 })
    e.preventDefault()
  }

  function onPointerMove(e: PointerEvent<HTMLDivElement>) {
    const start = startRef.current
    const el = layerRef.current
    if (!start || !el) return
    setDraft(fromCorners(start, toLocal(el, e)))
  }

  function finish(e: PointerEvent<HTMLDivElement>) {
    const start = startRef.current
    const el = layerRef.current
    startRef.current = null
    setDraft(null)
    if (!start || !el) return
    const next = fromCorners(start, toLocal(el, e))
    const layer = el.getBoundingClientRect()
    if (next.w * layer.width < 8 || next.h * layer.height < 8) return
    if (next.w < 0.01 || next.h < 0.01) return
    onDrawn(next)
  }

  return (
    <div className="roi-picker">
      <div className="roi-stage">
        <img src={src} alt={alt} draggable={false} />
        <div
          ref={layerRef}
          className={readOnly ? 'roi-layer readonly' : 'roi-layer'}
          role="application"
          aria-label={readOnly ? 'Sichtungen' : 'Graffiti umrahmen'}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={finish}
          onPointerCancel={() => {
            startRef.current = null
            setDraft(null)
          }}
        >
          {boxes.map((box) =>
            box.roi && box.roi.w > 0 && box.roi.h > 0 ? (
              <div
                key={box.id}
                data-box-id={box.id}
                className={box.id === selectedId ? 'roi-box selected' : 'roi-box'}
                style={boxStyle(box.roi)}
              />
            ) : null,
          )}
          {draft && draft.w > 0 && draft.h > 0 ? (
            <div className="roi-box draft" style={boxStyle(draft)} />
          ) : null}
        </div>
      </div>
    </div>
  )
}

export function RoiPicker({
  src,
  value,
  onChange,
  boxes,
  selectedId,
  onSelect,
  onDrawn,
  adding,
  hint,
  alt,
  readOnly = false,
}: Props) {
  const multi = boxes != null
  const shown = multi ? boxes : [{ id: '_', roi: value ?? null }]
  const selected = multi ? selectedId : '_'

  function handleDrawn(roi: Roi) {
    if (multi) onDrawn?.(roi)
    else onChange?.(roi)
  }

  const defaultHint = value
    ? 'Ausschnitt gesetzt. Mit Speichern übernimmt die Erkennung nur diesen Bereich.'
    : 'Graffiti auf dem Bild umrahmen. Das Foto bleibt vollständig; nur die Suche nutzt den Ausschnitt.'

  return (
    <div className="roi-field">
      <RoiCanvas
        src={src}
        boxes={shown}
        selectedId={selected}
        onSelect={onSelect}
        onDrawn={handleDrawn}
        readOnly={readOnly}
        alt={alt}
      />
      <p className="roi-hint">
        {hint ??
          (readOnly
            ? 'Rahmen anklicken, um die Sichtung zu wählen. Zum Ändern auf Bearbeiten.'
            : adding
              ? 'Nächstes Graffiti umrahmen.'
              : multi
                ? 'Graffiti umrahmen oder einen vorhandenen Rahmen anklicken. Weitere Sichtungen über den Button rechts.'
                : defaultHint)}
      </p>
      {!readOnly && !multi && value && onChange ? (
        <button type="button" className="linkish" onClick={() => onChange(null)}>
          Ausschnitt entfernen
        </button>
      ) : null}
    </div>
  )
}
