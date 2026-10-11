package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type nominatimHit struct {
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	DisplayName string            `json:"display_name"`
	Address     map[string]string `json:"address"`
}

type geoAddress struct {
	Street     string `json:"street,omitempty"`
	City       string `json:"city,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"`
	Label      string `json:"label,omitempty"`
}

const (
	geocodeMinGap          = time.Second
	geocodeHTTPTimeout     = 5 * time.Second
	addressReuseMaxMeters  = 5.0
	earthRadiusMeters      = 6371000.0
)

var (
	geocodeMu     sync.Mutex
	lastGeocode   time.Time
	geocodeClient = &http.Client{Timeout: geocodeHTTPTimeout}

	// Letzte erfolgreich aufgelöste Adresse — bei Serien am gleichen Spot
	// (≤ 5 m) Nominatim überspringen.
	addrCacheMu  sync.Mutex
	addrCacheOK  bool
	addrCacheLat float64
	addrCacheLon float64
	addrCache    map[string]any
)

func (s *Server) geocode(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "Bitte eine Adresse eingeben")
		return
	}
	if utf8.RuneCountInString(q) > 200 {
		writeError(w, http.StatusBadRequest, "Suche zu lang")
		return
	}

	if err := waitGeocodeSlot(r.Context()); err != nil {
		writeError(w, http.StatusGatewayTimeout, "Adresssuche abgebrochen")
		return
	}

	u := "https://nominatim.openstreetmap.org/search?format=jsonv2&limit=1&addressdetails=1&q=" + url.QueryEscape(q)
	hit, err := fetchNominatimHit(r.Context(), u)
	if err != nil {
		writeGeocodeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, geoResult(hit))
}

func (s *Server) reverseGeocode(w http.ResponseWriter, r *http.Request) {
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(r.URL.Query().Get("lat")), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(r.URL.Query().Get("lon")), 64)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "Koordinaten ungültig")
		return
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		writeError(w, http.StatusBadRequest, "Koordinaten ungültig")
		return
	}

	if err := waitGeocodeSlot(r.Context()); err != nil {
		writeError(w, http.StatusGatewayTimeout, "Adresssuche abgebrochen")
		return
	}

	u := "https://nominatim.openstreetmap.org/reverse?format=jsonv2&addressdetails=1&lat=" +
		strconv.FormatFloat(lat, 'f', 8, 64) + "&lon=" + strconv.FormatFloat(lon, 'f', 8, 64)
	hit, err := fetchNominatimHit(r.Context(), u)
	if err != nil {
		writeGeocodeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, geoResult(hit))
}

type geocodeError struct {
	status int
	msg    string
}

func (e *geocodeError) Error() string { return e.msg }

func writeGeocodeError(w http.ResponseWriter, err error) {
	var ge *geocodeError
	if errors.As(err, &ge) {
		writeError(w, ge.status, ge.msg)
		return
	}
	writeError(w, http.StatusBadGateway, "Adresssuche fehlgeschlagen")
}

func fetchNominatimHit(ctx context.Context, rawURL string) (nominatimHit, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nominatimHit{}, &geocodeError{http.StatusInternalServerError, "Suche fehlgeschlagen"}
	}
	req.Header.Set("User-Agent", "Schmutzfink/1.0 (graffiti-inventar)")
	req.Header.Set("Accept-Language", "de")

	res, err := geocodeClient.Do(req)
	if err != nil {
		return nominatimHit{}, &geocodeError{http.StatusBadGateway, "Adresssuche nicht erreichbar"}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nominatimHit{}, &geocodeError{http.StatusBadGateway, "Adresssuche nicht erreichbar"}
	}
	if res.StatusCode == http.StatusNotFound {
		return nominatimHit{}, &geocodeError{http.StatusNotFound, "Adresse nicht gefunden"}
	}
	if res.StatusCode != http.StatusOK {
		return nominatimHit{}, &geocodeError{http.StatusBadGateway, "Adresssuche fehlgeschlagen"}
	}

	// search returns an array; reverse returns a single object
	var hits []nominatimHit
	if err := json.Unmarshal(body, &hits); err == nil {
		if len(hits) == 0 {
			return nominatimHit{}, &geocodeError{http.StatusNotFound, "Adresse nicht gefunden"}
		}
		return hits[0], nil
	}
	var one nominatimHit
	if err := json.Unmarshal(body, &one); err != nil {
		return nominatimHit{}, &geocodeError{http.StatusBadGateway, "Adresssuche fehlgeschlagen"}
	}
	if strings.TrimSpace(one.Lat) == "" || strings.TrimSpace(one.Lon) == "" {
		return nominatimHit{}, &geocodeError{http.StatusNotFound, "Adresse nicht gefunden"}
	}
	return one, nil
}

func geoResult(hit nominatimHit) map[string]any {
	lat, err1 := strconv.ParseFloat(hit.Lat, 64)
	lon, err2 := strconv.ParseFloat(hit.Lon, 64)
	addr := addressFromNominatim(hit.DisplayName, hit.Address)
	out := map[string]any{
		"label":   addr.Label,
		"address": addr,
	}
	if err1 == nil && err2 == nil {
		out["lat"] = lat
		out["lon"] = lon
	}
	return out
}

func addressFromNominatim(display string, parts map[string]string) geoAddress {
	addr := geoAddress{
		Label:      strings.TrimSpace(display),
		Street:     firstNonEmpty(parts, "road", "pedestrian", "path", "footway", "cycleway", "square", "neighbourhood"),
		City:       firstNonEmpty(parts, "city", "town", "village", "municipality", "city_district", "suburb"),
		PostalCode: strings.TrimSpace(parts["postcode"]),
		Country:    strings.TrimSpace(parts["country"]),
	}
	if house := strings.TrimSpace(parts["house_number"]); house != "" && addr.Street != "" {
		addr.Street = addr.Street + " " + house
	}
	if addr.Label == "" {
		addr.Label = formatGeoAddress(addr)
	}
	return addr
}

func firstNonEmpty(parts map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(parts[k]); v != "" {
			return v
		}
	}
	return ""
}

func formatGeoAddress(addr geoAddress) string {
	var loc string
	switch {
	case addr.PostalCode != "" && addr.City != "":
		loc = addr.PostalCode + " " + addr.City
	case addr.City != "":
		loc = addr.City
	default:
		loc = addr.PostalCode
	}
	switch {
	case addr.Street != "" && loc != "":
		return addr.Street + ", " + loc
	case addr.Street != "":
		return addr.Street
	default:
		return loc
	}
}

func parseAddressJSON(raw string) (map[string]any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	var addr geoAddress
	if err := json.Unmarshal([]byte(raw), &addr); err != nil {
		return nil, false
	}
	out := map[string]any{}
	if v := strings.TrimSpace(addr.Street); v != "" {
		out["street"] = v
	}
	if v := strings.TrimSpace(addr.City); v != "" {
		out["city"] = v
	}
	if v := strings.TrimSpace(addr.PostalCode); v != "" {
		out["postal_code"] = v
	}
	if v := strings.TrimSpace(addr.Country); v != "" {
		out["country"] = v
	}
	if v := strings.TrimSpace(addr.Label); v != "" {
		out["label"] = v
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func addressFromExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	raw, ok := extra["address"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"street", "city", "postal_code", "country", "label"} {
		if v, ok := raw[key].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				out[key] = s
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func setExtraAddress(extra map[string]any, addr map[string]any) map[string]any {
	if extra == nil {
		extra = map[string]any{}
	}
	if addr == nil {
		delete(extra, "address")
		return extra
	}
	extra["address"] = addr
	return extra
}

func lookupAddress(ctx context.Context, lat, lon float64) map[string]any {
	if cached := cachedAddressNear(lat, lon); cached != nil {
		return cached
	}
	if err := waitGeocodeSlot(ctx); err != nil {
		return nil
	}
	u := "https://nominatim.openstreetmap.org/reverse?format=jsonv2&addressdetails=1&lat=" +
		strconv.FormatFloat(lat, 'f', 8, 64) + "&lon=" + strconv.FormatFloat(lon, 'f', 8, 64)
	hit, err := fetchNominatimHit(ctx, u)
	if err != nil {
		return nil
	}
	addr := addressFromNominatim(hit.DisplayName, hit.Address)
	raw, err := json.Marshal(addr)
	if err != nil {
		return nil
	}
	parsed, ok := parseAddressJSON(string(raw))
	if !ok {
		return nil
	}
	rememberResolvedAddress(lat, lon, parsed)
	return parsed
}

// rememberResolvedAddress merkt Koordinaten+Adresse als Basis für die nächste Näheprüfung.
func rememberResolvedAddress(lat, lon float64, addr map[string]any) {
	if len(addr) == 0 {
		return
	}
	addrCacheMu.Lock()
	defer addrCacheMu.Unlock()
	addrCacheLat = lat
	addrCacheLon = lon
	addrCache = copyAddressMap(addr)
	addrCacheOK = true
}

func cachedAddressNear(lat, lon float64) map[string]any {
	addrCacheMu.Lock()
	defer addrCacheMu.Unlock()
	if !addrCacheOK || len(addrCache) == 0 {
		return nil
	}
	if haversineMeters(addrCacheLat, addrCacheLon, lat, lon) > addressReuseMaxMeters {
		return nil
	}
	return copyAddressMap(addrCache)
}

func copyAddressMap(addr map[string]any) map[string]any {
	out := make(map[string]any, len(addr))
	for k, v := range addr {
		out[k] = v
	}
	return out
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(h)))
}

func resetAddressCacheForTest() {
	addrCacheMu.Lock()
	defer addrCacheMu.Unlock()
	addrCacheOK = false
	addrCache = nil
	addrCacheLat = 0
	addrCacheLon = 0
}

func waitGeocodeSlot(ctx context.Context) error {
	geocodeMu.Lock()
	wait := geocodeMinGap - time.Since(lastGeocode)
	if wait < 0 {
		wait = 0
	}
	lastGeocode = time.Now().Add(wait)
	geocodeMu.Unlock()
	if wait == 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
