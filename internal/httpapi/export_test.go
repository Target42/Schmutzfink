package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"schmutzfink/internal/records"
)

func TestAnonymousExportIsUnauthorized(t *testing.T) {
	h := (&Server{}).Router()
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/records/export?format=csv", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s /api/records/export: got %d, want 401", method, rec.Code)
		}
	}
}

func TestParseExportFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: "json"},
		{in: "json", want: "json"},
		{in: "CSV", want: "csv"},
		{in: "xml", wantErr: true},
	}
	for _, tc := range cases {
		got, err := parseExportFormat(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseExportFormat(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("parseExportFormat(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestPresentExportItemOmitsFilesAndUserIDs(t *testing.T) {
	lat, lon, score := 52.375, 9.732, 0.81
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	got := presentExportItem(records.Record{
		ID:               "s1",
		RecordID:         "r1",
		Note:             "Tag an der Wand",
		Tags:             []string{"rot", "groß"},
		Lat:              &lat,
		Lon:              &lon,
		CapturedAt:       &at,
		UploadedAt:       at,
		UploadedBy:       "user-id",
		UploadedByName:   "anna",
		ContentType:      "image/jpeg",
		OriginalObjectID: "obj-1",
		ThumbObjectID:    "thumb-1",
		EmbeddingStatus:  "ready",
		Score:            &score,
		MotifID:          "m1",
		MotifTitle:       "Drache",
		Fields:           map[string]string{"bezirk": "Nord"},
		ROI:              &records.ROI{X: 0.1, Y: 0.2, W: 0.3, H: 0.4},
	})
	if got.SightingID != "s1" || got.RecordID != "r1" {
		t.Fatalf("ids: %+v", got)
	}
	if got.UploadedByName != "anna" {
		t.Fatalf("uploaded_by_name: %q", got.UploadedByName)
	}
	if got.Score == nil || *got.Score != score {
		t.Fatalf("score: %v", got.Score)
	}
	if got.Fields["bezirk"] != "Nord" {
		t.Fatalf("fields: %v", got.Fields)
	}
	raw := presentSightingHit(records.Record{ID: "s1", RecordID: "r1"})
	if _, ok := raw["thumb_url"]; !ok {
		t.Fatal("list presenter should still expose thumb_url")
	}
}

func TestWriteCSVExport(t *testing.T) {
	lat := 52.3759
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	rec := httptest.NewRecorder()
	writeCSVExport(rec, []exportItem{{
		SightingID:     "s1",
		RecordID:       "r1",
		Note:           "Tag; an der Wand",
		Tags:           []string{"rot", "groß"},
		Lat:            &lat,
		UploadedAt:     at,
		UploadedByName: "anna",
		MotifTitle:     "Drache",
		Fields:         map[string]string{"bezirk": "Nord"},
	}}, []records.FieldDef{{Key: "bezirk", Label: "Bezirk"}})

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("content-type: %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, "field.bezirk") {
		t.Fatalf("missing field column: %s", body)
	}
	if !strings.Contains(body, "Nord") || !strings.Contains(body, "rot, groß") {
		t.Fatalf("row: %s", body)
	}
	if strings.Contains(body, "thumb_url") || strings.Contains(body, "/original") {
		t.Fatalf("export leaked file urls: %s", body)
	}
}
