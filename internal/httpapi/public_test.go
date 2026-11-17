package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"schmutzfink/internal/records"
)

func TestPresentPublicPointsOmitsDetails(t *testing.T) {
	lat, lon := 52.5, 13.4
	got := presentPublicPoints([]records.Record{
		{
			ID:         "sight-1",
			RecordID:   "photo-1",
			Note:       "geheime notiz",
			Tags:       []string{"tag"},
			Lat:        &lat,
			Lon:        &lon,
			UploadedBy: "anna",
			ROI:        &records.ROI{X: 0.1, Y: 0.2, W: 0.3, H: 0.4},
		},
		{ID: "no-gps", Note: "ohne ort", RecordID: "photo-2"},
	})
	if len(got) != 1 {
		t.Fatalf("points: %d", len(got))
	}
	point := got[0]
	for _, key := range []string{"note", "tags", "record_id", "uploaded_by", "original_url", "fields"} {
		if _, ok := point[key]; ok {
			t.Fatalf("public point leaked %s", key)
		}
	}
	if point["id"] != "sight-1" || point["lat"] != &lat || point["lon"] != &lon {
		t.Fatalf("point: %#v", point)
	}
	thumb, _ := point["thumb_url"].(string)
	if !strings.HasPrefix(thumb, "/api/public/sightings/sight-1/thumb?") {
		t.Fatalf("thumb: %s", thumb)
	}
	if strings.Contains(thumb, "/original") || strings.Contains(thumb, "/api/records/") {
		t.Fatalf("thumb points at a full record: %s", thumb)
	}
}

func TestPublicMapStaysClosedWithoutDatabase(t *testing.T) {
	h := (&Server{}).Router()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/public/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("config: %d", rec.Code)
	}
	var cfg struct {
		PublicMap bool `json:"public_map"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.PublicMap {
		t.Fatal("public map must default off")
	}
	if rec.Header().Get("X-Robots-Tag") == "" {
		t.Fatal("public config should ask crawlers to stay away")
	}

	for _, path := range []string{
		"/api/public/map",
		"/api/public/sightings/00000000-0000-4000-8000-000000000099/thumb",
		"/api/sightings/00000000-0000-4000-8000-000000000099/thumb",
		"/api/records/00000000-0000-4000-8000-000000000099/original",
		"/api/settings",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		want := http.StatusNotFound
		if path == "/api/sightings/00000000-0000-4000-8000-000000000099/thumb" ||
			path == "/api/records/00000000-0000-4000-8000-000000000099/original" ||
			path == "/api/settings" {
			want = http.StatusUnauthorized
		}
		if rec.Code != want {
			t.Errorf("%s: got %d, want %d", path, rec.Code, want)
		}
	}
}
