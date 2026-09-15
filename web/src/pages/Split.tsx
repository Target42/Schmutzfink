import {
  Children,
  useCallback,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
  type ReactNode,
} from 'react'

type Axis = 'x' | 'y'

type Props = {
  id: string
  axis: Axis
  defaultPct?: number
  minA?: number
  minB?: number
  className?: string
  children: ReactNode
}

function loadPct(id: string, fallback: number) {
  try {
    const n = Number(localStorage.getItem(id))
    if (Number.isFinite(n) && n >= 8 && n <= 92) return n
  } catch {
    /* private mode */
  }
  return fallback
}

function savePct(id: string, pct: number) {
  try {
    localStorage.setItem(id, String(Math.round(pct * 10) / 10))
  } catch {
    /* private mode */
  }
}

export function Split({
  id,
  axis,
  defaultPct = 56,
  minA = 160,
  minB = 160,
  className,
  children,
}: Props) {
  const panes = Children.toArray(children)
  const [pct, setPct] = useState(() => loadPct(id, defaultPct))
  const [dragging, setDragging] = useState(false)
  const boxRef = useRef<HTMLDivElement>(null)

  const apply = useCallback(
    (next: number, persist: boolean) => {
      const clamped = Math.min(92, Math.max(8, next))
      setPct(clamped)
      if (persist) savePct(id, clamped)
    },
    [id],
  )

  function clampToMins(raw: number, size: number) {
    const lo = Math.max(8, (minA / size) * 100)
    const hi = Math.min(92, 100 - (minB / size) * 100)
    if (lo >= hi) return 50
    return Math.min(hi, Math.max(lo, raw))
  }

  function moveTo(clientX: number, clientY: number, persist: boolean) {
    const box = boxRef.current
    if (!box) return
    const r = box.getBoundingClientRect()
    const size = axis === 'x' ? r.width : r.height
    if (size < 8) return
    const pos = axis === 'x' ? clientX - r.left : clientY - r.top
    apply(clampToMins((pos / size) * 100, size), persist)
  }

  function onPointerDown(e: PointerEvent<HTMLDivElement>) {
    if (e.button !== 0) return
    e.preventDefault()
    e.currentTarget.setPointerCapture(e.pointerId)
    setDragging(true)
    moveTo(e.clientX, e.clientY, false)
  }

  function onPointerMove(e: PointerEvent<HTMLDivElement>) {
    if (!e.currentTarget.hasPointerCapture(e.pointerId)) return
    moveTo(e.clientX, e.clientY, false)
  }

  function endDrag(e: PointerEvent<HTMLDivElement>) {
    if (!e.currentTarget.hasPointerCapture(e.pointerId)) return
    e.currentTarget.releasePointerCapture(e.pointerId)
    setDragging(false)
    moveTo(e.clientX, e.clientY, true)
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const step = e.shiftKey ? 8 : 2
    if (axis === 'x' && e.key === 'ArrowLeft') {
      e.preventDefault()
      apply(pct - step, true)
    } else if (axis === 'x' && e.key === 'ArrowRight') {
      e.preventDefault()
      apply(pct + step, true)
    } else if (axis === 'y' && e.key === 'ArrowUp') {
      e.preventDefault()
      apply(pct - step, true)
    } else if (axis === 'y' && e.key === 'ArrowDown') {
      e.preventDefault()
      apply(pct + step, true)
    } else if (e.key === 'Home') {
      e.preventDefault()
      apply(8, true)
    } else if (e.key === 'End') {
      e.preventDefault()
      apply(92, true)
    }
  }

  if (panes.length < 2) return <>{children}</>

  const cls = ['split-pane', axis === 'x' ? 'split-x' : 'split-y', dragging ? 'is-dragging' : '', className]
    .filter(Boolean)
    .join(' ')

  return (
    <div ref={boxRef} className={cls}>
      <div className="split-pane-a" style={{ flex: `0 0 ${pct}%` }}>
        {panes[0]}
      </div>
      <div
        className="split-gutter"
        data-axis={axis}
        role="separator"
        tabIndex={0}
        aria-orientation={axis === 'x' ? 'vertical' : 'horizontal'}
        aria-valuenow={Math.round(pct)}
        aria-valuemin={8}
        aria-valuemax={92}
        aria-label={axis === 'x' ? 'Breite der Spalten anpassen' : 'Höhe der Bereiche anpassen'}
        title="Ziehen zum Anpassen. Doppelklick setzt zurück."
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onKeyDown={onKeyDown}
        onDoubleClick={() => apply(defaultPct, true)}
      />
      <div className="split-pane-b">{panes[1]}</div>
    </div>
  )
}
