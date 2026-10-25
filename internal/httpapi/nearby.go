package httpapi

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"schmutzfink/internal/records"
)

func (s *Server) nearbyRecords(w http.ResponseWriter, r *http.Request) {
	lat, lon, radius, err := parseNearbyQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hits, err := s.Records.NearbyHits(r.Context(), principal(r).TenantID, lat, lon, radius, 80)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Funde in der Nähe konnten nicht geladen werden")
		return
	}
	places := records.GroupNearby(hits, time.Now().UTC())
	for i := range places {
		if places[i].SightingID != "" {
			places[i].ThumbURL = "/api/sightings/" + places[i].SightingID + "/thumb"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":    places,
		"radius_m": radius,
	})
}

func parseNearbyQuery(q url.Values) (lat, lon, radius float64, err error) {
	lat, err = strconv.ParseFloat(q.Get("lat"), 64)
	if err != nil || lat < -90 || lat > 90 {
		return 0, 0, 0, errNearby("lat ungültig")
	}
	lon, err = strconv.ParseFloat(q.Get("lon"), 64)
	if err != nil || lon < -180 || lon > 180 {
		return 0, 0, 0, errNearby("lon ungültig")
	}
	radius, err = strconv.ParseFloat(q.Get("radius_m"), 64)
	if err != nil || radius < 10 || radius > 200 {
		return 0, 0, 0, errNearby("radius_m muss zwischen 10 und 200 liegen")
	}
	return lat, lon, radius, nil
}

type nearbyError string

func (e nearbyError) Error() string { return string(e) }

func errNearby(msg string) error { return nearbyError(msg) }
