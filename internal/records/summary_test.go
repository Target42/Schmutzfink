package records

import (
	"testing"
	"time"
)

func TestSummarizeCounts(t *testing.T) {
	lat, lon := 52.5, 13.4
	captured := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	items := []Record{
		{
			ID: "s1", RecordID: "r1", Tags: []string{"Kreuzberg", "Mural"},
			Lat: &lat, Lon: &lon, CapturedAt: &captured, UploadedAt: captured,
			MotifID: "m1", MotifTitle: "Smiley", Fields: map[string]string{"gangname": "Oz"},
		},
		{
			ID: "s2", RecordID: "r1", Tags: []string{"Kreuzberg"},
			Lat: &lat, Lon: &lon, UploadedAt: captured,
			Fields: map[string]string{"gangname": "Oz"},
		},
		{
			ID: "s3", RecordID: "r2", Tags: []string{" "},
			UploadedAt: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC),
		},
	}
	sum := Summarize(items, 5, []FieldDef{{Key: "gangname", Label: "Gangname"}})
	if sum.Sightings != 3 || sum.Photos != 2 || sum.WithGPS != 2 || sum.WithoutGPS != 1 || sum.WithMotif != 1 {
		t.Fatalf("totals: %+v", sum)
	}
	if !sum.Truncated || sum.Total != 5 {
		t.Fatalf("truncated: %+v", sum)
	}
	if len(sum.Months) != 2 || sum.Months[0].Key != "2026-04" || sum.Months[0].Count != 2 {
		t.Fatalf("months: %+v", sum.Months)
	}
	if len(sum.Tags) == 0 || sum.Tags[0].Key != "Kreuzberg" || sum.Tags[0].Count != 2 {
		t.Fatalf("tags: %+v", sum.Tags)
	}
	if len(sum.Motifs) < 2 || sum.Motifs[0].Label != "ohne Motiv" {
		t.Fatalf("motifs: %+v", sum.Motifs)
	}
	if len(sum.Fields) != 1 || sum.Fields[0].Values[0].Count != 2 {
		t.Fatalf("fields: %+v", sum.Fields)
	}
	if len(sum.Hotspots) != 1 || sum.Hotspots[0].Count != 2 {
		t.Fatalf("hotspots: %+v", sum.Hotspots)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	sum := Summarize(nil, 0, nil)
	if sum.Sightings != 0 || sum.Months == nil || sum.Hotspots == nil {
		t.Fatalf("empty slices: %+v", sum)
	}
}
