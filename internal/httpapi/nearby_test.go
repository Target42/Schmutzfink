package httpapi

import (
	"net/url"
	"testing"
)

func TestParseNearbyQuery(t *testing.T) {
	q := url.Values{}
	q.Set("lat", "52.37")
	q.Set("lon", "9.73")
	q.Set("radius_m", "50")
	lat, lon, radius, err := parseNearbyQuery(q)
	if err != nil || lat != 52.37 || lon != 9.73 || radius != 50 {
		t.Fatalf("%v %v %v %v", lat, lon, radius, err)
	}
	q.Set("radius_m", "5000")
	if _, _, _, err := parseNearbyQuery(q); err == nil {
		t.Fatal("expected radius error")
	}
}
