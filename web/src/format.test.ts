import { describe, expect, it } from 'vitest'
import { formatCoords, formatRadius, parseCoord } from './format.ts'

describe('parseCoord', () => {
  it('reads comma decimals', () => {
    expect(parseCoord('52,5')).toBe(52.5)
  })
  it('returns null for empty or junk', () => {
    expect(parseCoord('')).toBeNull()
    expect(parseCoord('abc')).toBeNull()
  })
})

describe('formatCoords', () => {
  it('labels missing GPS', () => {
    expect(formatCoords(null, null)).toBe('ohne GPS')
  })
  it('formats five decimals', () => {
    expect(formatCoords(52.5, 13.4)).toBe('52.50000, 13.40000')
  })
})

describe('formatRadius', () => {
  it('uses metres and whole kilometres', () => {
    expect(formatRadius(500)).toBe('500 m')
    expect(formatRadius(1000)).toBe('1 km')
  })
})
