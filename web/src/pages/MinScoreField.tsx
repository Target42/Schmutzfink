import { useState } from 'react'
import { loadMinScore, minScorePresets, saveMinScore } from '../types.ts'

type Props = {
  value: number
  onChange: (score: number) => void
}

export function MinScoreField({ value, onChange }: Props) {
  return (
    <label>
      Mindestähnlichkeit
      <select
        value={String(value)}
        onChange={(e) => {
          const score = Number(e.target.value)
          saveMinScore(score)
          onChange(score)
        }}
      >
        {minScorePresets.map((p) => (
          <option key={p.score} value={p.score}>
            {p.label}
          </option>
        ))}
      </select>
    </label>
  )
}

export function useMinScore() {
  const [value, setValue] = useState(loadMinScore)
  return [
    value,
    (score: number) => {
      saveMinScore(score)
      setValue(score)
    },
  ] as const
}
