package records

import (
	"testing"
	"time"
)

func TestGroupNearby(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	old := now.Add(-120 * 24 * time.Hour)
	recent := now.Add(-10 * 24 * time.Hour)
	places := GroupNearby([]NearbyHit{
		{
			SightingID: "s-far", RecordID: "r1", MotifID: "m1", MotifTitle: "Drache",
			MotifLastAt: &old, DistanceM: 40, SeenAt: old, CaseID: "c1", CaseTitle: "Akte",
		},
		{
			SightingID: "s-near", RecordID: "r2", MotifID: "m1", MotifTitle: "Drache",
			MotifLastAt: &old, DistanceM: 8, SeenAt: old.Add(24 * time.Hour),
		},
		{
			SightingID: "s-new", RecordID: "r3", MotifID: "m2", MotifTitle: "Smiley",
			MotifLastAt: &recent, DistanceM: 4, SeenAt: recent,
		},
		{
			SightingID: "s-lone", RecordID: "r4", Note: "Wand", DistanceM: 15, SeenAt: old,
		},
	}, now)
	if len(places) != 3 {
		t.Fatalf("places: %+v", places)
	}
	if places[0].MotifID != "m1" || !places[0].FollowUp || places[0].Count != 2 {
		t.Fatalf("follow-up first: %+v", places[0])
	}
	if places[0].SightingID != "s-near" || places[0].DistanceM != 8 || places[0].CaseID != "c1" {
		t.Fatalf("nearest sighting and case: %+v", places[0])
	}
	if places[1].MotifID != "m2" || places[1].FollowUp {
		t.Fatalf("recent motif: %+v", places[1])
	}
	if places[2].Key != "record:r4" || places[2].FollowUp || places[2].Title != "Wand" {
		t.Fatalf("lone: %+v", places[2])
	}
}
