import { useEffect, useRef, useState } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import markerIcon from 'leaflet/dist/images/marker-icon.png'
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png'
import markerShadow from 'leaflet/dist/images/marker-shadow.png'

const icon = L.icon({
  iconUrl: markerIcon,
  iconRetinaUrl: markerIcon2x,
  shadowUrl: markerShadow,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  tooltipAnchor: [0, -34],
})

const centerIcon = L.divIcon({
  className: 'radius-center-icon',
  iconSize: [16, 16],
  iconAnchor: [8, 8],
})

export type MapPoint = {
  id: string
  recordId?: string
  lat: number
  lon: number
  note?: string
  thumb_url?: string
  kind?: 'current' | 'similar'
}

const similarIcon = L.divIcon({
  className: 'similar-flag',
  html: '<span></span>',
  iconSize: [18, 18],
  iconAnchor: [9, 17],
  popupAnchor: [0, -16],
  tooltipAnchor: [0, -16],
})

const thumbTipOpts: L.TooltipOptions = {
  direction: 'top',
  offset: [0, -4],
  opacity: 1,
  className: 'map-thumb-tip',
  interactive: false,
}

export type SearchArea = {
  lat: number
  lon: number
  radiusM: number
}

type Props = {
  points: MapPoint[]
  className?: string
  popups?: boolean
  focusId?: string
  searchArea?: SearchArea | null
  onPickCenter?: (lat: number, lon: number) => void
}

export function GraffitiMap({
  points,
  className,
  popups = true,
  focusId,
  searchArea,
  onPickCenter,
}: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const mapRef = useRef<L.Map | null>(null)
  const focusedRef = useRef<string | null>(null)
  const onPickRef = useRef(onPickCenter)
  const areaRef = useRef<{ circle: L.Circle; marker: L.Marker } | null>(null)
  const [ready, setReady] = useState(false)
  const hasArea = searchArea != null
  const pointsKey = points.map((p) => `${p.id}:${p.lat}:${p.lon}:${p.kind ?? ''}:${p.thumb_url ?? ''}`).join('|')
  onPickRef.current = onPickCenter

  useEffect(() => {
    if (!ref.current) return
    const map = L.map(ref.current).setView([51.16, 10.45], 6)
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map)
    map.createPane('searchArea')
    const pane = map.getPane('searchArea')
    if (pane) pane.style.zIndex = '350'
    mapRef.current = map
    setReady(true)
    let resizeT = 0
    const ro = new ResizeObserver(() => {
      window.clearTimeout(resizeT)
      resizeT = window.setTimeout(() => map.invalidateSize(), 80)
    })
    ro.observe(ref.current)
    const t = window.setTimeout(() => {
      map.invalidateSize()
      if (areaRef.current) {
        map.fitBounds(areaRef.current.circle.getBounds(), { padding: [40, 40], maxZoom: 17 })
      }
    }, 80)
    return () => {
      window.clearTimeout(t)
      window.clearTimeout(resizeT)
      ro.disconnect()
      map.remove()
      mapRef.current = null
      areaRef.current = null
      setReady(false)
    }
  }, [])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    const onClick = (e: L.LeafletMouseEvent) => {
      onPickRef.current?.(e.latlng.lat, e.latlng.lng)
    }
    map.on('click', onClick)
    return () => {
      map.off('click', onClick)
    }
  }, [ready])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    if (!searchArea) {
      if (areaRef.current) {
        map.removeLayer(areaRef.current.circle)
        map.removeLayer(areaRef.current.marker)
        areaRef.current = null
      }
      return
    }
    const ll = L.latLng(searchArea.lat, searchArea.lon)
    let area = areaRef.current
    if (!area) {
      const circle = L.circle(ll, {
        radius: searchArea.radiusM,
        pane: 'searchArea',
        color: '#b4532a',
        weight: 2,
        fillColor: '#b4532a',
        fillOpacity: 0.12,
        interactive: false,
      }).addTo(map)
      const marker = L.marker(ll, { icon: centerIcon, draggable: Boolean(onPickRef.current), zIndexOffset: 600 })
      marker.on('drag', () => {
        circle.setLatLng(marker.getLatLng())
      })
      marker.on('dragend', () => {
        const p = marker.getLatLng()
        onPickRef.current?.(p.lat, p.lng)
      })
      marker.addTo(map)
      area = { circle, marker }
      areaRef.current = area
    } else {
      area.circle.setLatLng(ll)
      area.circle.setRadius(searchArea.radiusM)
      area.marker.setLatLng(ll)
    }
    map.fitBounds(area.circle.getBounds(), { padding: [40, 40], maxZoom: 17 })
  }, [ready, searchArea?.lat, searchArea?.lon, searchArea?.radiusM])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    const layer = L.layerGroup().addTo(map)
    const latlngs: L.LatLngExpression[] = []
    const markers = new Map<string, L.Marker>()
    for (const p of points) {
      const m = L.marker([p.lat, p.lon], {
        icon: p.kind === 'similar' ? similarIcon : icon,
        zIndexOffset: p.kind === 'current' ? 500 : 0,
      })
      if (searchArea) {
        const d = L.latLng(p.lat, p.lon).distanceTo(L.latLng(searchArea.lat, searchArea.lon))
        if (d > searchArea.radiusM) m.setOpacity(0.4)
      }
      if (popups) {
        const img = p.thumb_url ? `<p><img src="${p.thumb_url}" alt="" width="160" /></p>` : ''
        const href = p.recordId ? `/datensatz/${p.recordId}?s=${p.id}` : `/datensatz/${p.id}`
        m.bindPopup(`${img}<a href="${href}">${escapeHtml(p.note || 'Sichtung')}</a>`)
      }
      if (p.thumb_url) bindThumbPreview(m, p.thumb_url)
      m.on('click', () => {
        onPickRef.current?.(p.lat, p.lon)
      })
      m.addTo(layer)
      markers.set(p.id, m)
      latlngs.push([p.lat, p.lon])
    }
    const focus = focusId ? markers.get(focusId) : undefined
    if (focus && focusId) {
      const ll = focus.getLatLng()
      if (focusedRef.current !== focusId) {
        const prev = focusedRef.current
        focusedRef.current = focusId
        if (prev) {
          map.flyTo(ll, Math.max(map.getZoom(), 14), { duration: 0.55 })
        } else {
          map.setView(ll, 15)
          if (popups) focus.openPopup()
        }
      }
    } else if (!hasArea) {
      if (latlngs.length === 1) map.setView(latlngs[0], 16)
      else if (latlngs.length > 1) map.fitBounds(L.latLngBounds(latlngs), { padding: [24, 24], maxZoom: 16 })
    }
    return () => {
      map.removeLayer(layer)
    }
  }, [pointsKey, popups, ready, focusId, hasArea, searchArea?.lat, searchArea?.lon, searchArea?.radiusM])

  const cls = [className ?? 'map', onPickCenter ? 'map-pick' : ''].filter(Boolean).join(' ')
  return <div ref={ref} className={cls} />
}

export function MiniMap({ lat, lon }: { lat: number; lon: number }) {
  return (
    <GraffitiMap
      className="map map-mini"
      popups={false}
      points={[{ id: `${lat},${lon}`, lat, lon, kind: 'current' }]}
    />
  )
}

function bindThumbPreview(marker: L.Marker, thumbUrl: string) {
  marker.bindTooltip(
    `<img src="${escapeHtml(thumbUrl)}" alt="" width="160" height="120" />`,
    thumbTipOpts,
  )
  marker.on('popupopen', () => {
    marker.closeTooltip()
  })
}

function escapeHtml(s: string) {
  return s.replace(/[&<>"']/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch] || ch)
}
