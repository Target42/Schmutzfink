export function formatWhen(iso: string | null | undefined) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('de-DE', { dateStyle: 'short', timeStyle: 'short' })
}

export function formatDay(iso: string | null | undefined) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleDateString('de-DE', { dateStyle: 'medium' })
}

export function formatCoords(lat: number | null, lon: number | null) {
  if (lat == null || lon == null) return 'ohne GPS'
  return `${lat.toFixed(5)}, ${lon.toFixed(5)}`
}

export function formatCoord(n: number) {
  return (Math.round(n * 1e6) / 1e6).toString()
}

export function parseCoord(raw: string): number | null {
  const t = raw.trim().replace(',', '.')
  if (!t) return null
  const n = Number(t)
  return Number.isFinite(n) ? n : null
}

export function formatRadius(m: number) {
  if (m >= 1000 && m % 1000 === 0) return `${m / 1000} km`
  if (m >= 1000) return `${String(m / 1000).replace('.', ',')} km`
  return `${m} m`
}

export function distanceMeters(lat1: number, lon1: number, lat2: number, lon2: number) {
  const toRad = (d: number) => (d * Math.PI) / 180
  const dLat = toRad(lat2 - lat1)
  const dLon = toRad(lon2 - lon1)
  const h =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2
  return 2 * 6371000 * Math.asin(Math.min(1, Math.sqrt(h)))
}
