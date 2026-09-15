package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type nominatimHit struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
}

const (
	geocodeMinGap      = time.Second
	geocodeHTTPTimeout = 5 * time.Second
)

var (
	geocodeMu     sync.Mutex
	lastGeocode   time.Time
	geocodeClient = &http.Client{Timeout: geocodeHTTPTimeout}
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

	u := "https://nominatim.openstreetmap.org/search?format=jsonv2&limit=1&q=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Suche fehlgeschlagen")
		return
	}
	req.Header.Set("User-Agent", "Schmutzfink/1.0 (graffiti-inventar)")
	req.Header.Set("Accept-Language", "de")

	res, err := geocodeClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Adresssuche nicht erreichbar")
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Adresssuche nicht erreichbar")
		return
	}
	if res.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "Adresssuche fehlgeschlagen")
		return
	}

	var hits []nominatimHit
	if err := json.Unmarshal(body, &hits); err != nil {
		writeError(w, http.StatusBadGateway, "Adresssuche fehlgeschlagen")
		return
	}
	if len(hits) == 0 {
		writeError(w, http.StatusNotFound, "Adresse nicht gefunden")
		return
	}
	lat, err1 := strconv.ParseFloat(hits[0].Lat, 64)
	lon, err2 := strconv.ParseFloat(hits[0].Lon, 64)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadGateway, "Adresssuche fehlgeschlagen")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lat":   lat,
		"lon":   lon,
		"label": hits[0].DisplayName,
	})
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
