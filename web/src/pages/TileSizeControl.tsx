import { useState } from 'react'
import { loadTileSize, saveTileSize, tileSizeLabels, tileSizes, type TileSize } from '../types.ts'

type Props = {
  value: TileSize
  onChange: (size: TileSize) => void
}

export function TileSizeControl({ value, onChange }: Props) {
  return (
    <div className="tile-size-control" role="group" aria-label="Kachelgröße">
      <span className="muted">Kacheln</span>
      <div className="tile-size-buttons">
        {tileSizes.map((size) => (
          <button
            key={size}
            type="button"
            className="secondary"
            aria-pressed={value === size}
            title={tileSizeLabels[size]}
            onClick={() => {
              saveTileSize(size)
              onChange(size)
            }}
          >
            {tileSizeLabels[size]}
          </button>
        ))}
      </div>
    </div>
  )
}

export function useTileSize() {
  const [value, setValue] = useState(loadTileSize)
  return [
    value,
    (size: TileSize) => {
      saveTileSize(size)
      setValue(size)
    },
  ] as const
}
