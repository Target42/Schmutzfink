import { useEffect, useRef, useState } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import markerIcon from 'leaflet/dist/images/marker-icon.png'
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png'
import markerShadow from 'leaflet/dist/images/marker-shadow.png'
import { api } from '../api.ts'

const icon = L.icon({
  iconUrl: markerIcon,
  iconRetinaUrl: markerIcon2x,
  shadowUrl: markerShadow,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
})

const germany: L.LatLngExpression = [51.16, 10.45]

type Props = {
  lat: number | null
  lon: number | null
  onChange: (lat: number, lon: number, via?: 'map' | 'search') => void
}

export function LocationPicker({ lat, lon, onChange }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const mapRef = useRef<L.Map | null>(null)
  const markerRef = useRef<L.Marker | null>(null)
  const onChangeRef = useRef(onChange)
  const pendingZoomRef = useRef<number | null>(null)
  const [ready, setReady] = useState(false)
  const [query, setQuery] = useState('')
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [found, setFound] = useState('')
  onChangeRef.current = onChange

  useEffect(() => {
    if (!ref.current) return
    const map = L.map(ref.current).setView(germany, 6)
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map)
    map.on('click', (e: L.LeafletMouseEvent) => {
      onChangeRef.current(e.latlng.lat, e.latlng.lng, 'map')
    })
    mapRef.current = map
    setReady(true)
    const ro = new ResizeObserver(() => map.invalidateSize())
    ro.observe(ref.current)
    const t = window.setTimeout(() => map.invalidateSize(), 80)
    return () => {
      window.clearTimeout(t)
      ro.disconnect()
      map.remove()
      mapRef.current = null
      markerRef.current = null
      setReady(false)
    }
  }, [])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    if (lat == null || lon == null) {
      if (markerRef.current) {
        map.removeLayer(markerRef.current)
        markerRef.current = null
      }
      return
    }
    const ll = L.latLng(lat, lon)
    const zoom = pendingZoomRef.current
    pendingZoomRef.current = null
    if (!markerRef.current) {
      const marker = L.marker(ll, { icon, draggable: true })
      marker.on('dragend', () => {
        const p = marker.getLatLng()
        onChangeRef.current(p.lat, p.lng, 'map')
      })
      marker.addTo(map)
      markerRef.current = marker
      map.setView(ll, zoom ?? 16)
      return
    }
    const current = markerRef.current.getLatLng()
    if (current.distanceTo(ll) < 0.05 && zoom == null) return
    markerRef.current.setLatLng(ll)
    if (zoom != null) map.setView(ll, zoom)
    else if (!map.getBounds().contains(ll)) map.panTo(ll)
  }, [lat, lon, ready])

  async function searchAddress() {
    const q = query.trim()
    if (!q) {
      setSearchError('Bitte eine Adresse eingeben')
      return
    }
    setSearching(true)
    setSearchError('')
    setFound('')
    try {
      const hit = await api.geocode(q)
      pendingZoomRef.current = 18
      mapRef.current?.setView([hit.lat, hit.lon], 18)
      onChange(hit.lat, hit.lon, 'search')
      setFound(hit.label)
    } catch (err) {
      setSearchError(err instanceof Error ? err.message : 'Adresse nicht gefunden')
    } finally {
      setSearching(false)
    }
  }

  return (
    <div className="location-picker">
      <div className="geo-search">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              void searchAddress()
            }
          }}
          placeholder="Adresse, z. B. Hannover, Vahrenwalder Straße 22"
          autoComplete="off"
        />
        <button type="button" className="secondary" disabled={searching} onClick={() => void searchAddress()}>
          {searching ? 'Suche …' : 'Suchen'}
        </button>
      </div>
      {searchError ? <p className="error">{searchError}</p> : null}
      {found ? <p className="gps-hint">Gefunden: {found}</p> : null}
      <div ref={ref} className="map map-picker" />
    </div>
  )
}
