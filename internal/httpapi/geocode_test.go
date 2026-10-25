package httpapi

import (
	"math"
	"testing"
)

func TestAddressFromNominatim(t *testing.T) {
	addr := addressFromNominatim(
		"Sundernstraße, Westerfeld, Hemmingen, 30966, Deutschland",
		map[string]string{
			"road":     "Sundernstraße",
			"suburb":   "Westerfeld",
			"town":     "Hemmingen",
			"postcode": "30966",
			"country":  "Deutschland",
		},
	)
	if addr.Street != "Sundernstraße" {
		t.Fatalf("street=%q", addr.Street)
	}
	if addr.City != "Hemmingen" {
		t.Fatalf("city=%q", addr.City)
	}
	if addr.PostalCode != "30966" || addr.Country != "Deutschland" {
		t.Fatalf("addr=%+v", addr)
	}
	if addr.Label == "" {
		t.Fatal("empty label")
	}
}

func TestAddressFromNominatimHouseNumber(t *testing.T) {
	addr := addressFromNominatim("", map[string]string{
		"road":         "Vahrenwalder Straße",
		"house_number": "22",
		"city":         "Hannover",
		"postcode":     "30165",
	})
	if addr.Street != "Vahrenwalder Straße 22" {
		t.Fatalf("street=%q", addr.Street)
	}
	if got := formatGeoAddress(addr); got != "Vahrenwalder Straße 22, 30165 Hannover" {
		t.Fatalf("format=%q", got)
	}
}

func TestParseAddressJSON(t *testing.T) {
	addr, ok := parseAddressJSON(`{"street":"Am Wall 1","city":"Hannover","postal_code":"30159","label":"Am Wall 1, Hannover"}`)
	if !ok || addr["city"] != "Hannover" || addr["label"] == nil {
		t.Fatalf("addr=%v ok=%v", addr, ok)
	}
	if _, ok := parseAddressJSON(`{}`); ok {
		t.Fatal("empty should fail")
	}
}

func TestAddressFromExtra(t *testing.T) {
	got := addressFromExtra(map[string]any{
		"address": map[string]any{"street": "Am Wall 1", "city": "Hannover", "noise": 1},
	})
	if got["street"] != "Am Wall 1" || got["city"] != "Hannover" {
		t.Fatalf("got=%v", got)
	}
	if addressFromExtra(map[string]any{"import": map[string]any{}}) != nil {
		t.Fatal("expected nil")
	}
}

func TestHaversineMeters(t *testing.T) {
	// ~111 m Nord-Süd pro 0.001° Breite.
	d := haversineMeters(52.0, 9.0, 52.001, 9.0)
	if math.Abs(d-111.2) > 2 {
		t.Fatalf("expected ~111 m, got %.1f", d)
	}
	if haversineMeters(52.0, 9.0, 52.0, 9.0) != 0 {
		t.Fatal("same point should be 0")
	}
}

func TestCachedAddressNearReuse(t *testing.T) {
	t.Cleanup(resetAddressCacheForTest)
	resetAddressCacheForTest()

	base := map[string]any{
		"street":      "Am Wall 1",
		"city":        "Hannover",
		"postal_code": "30159",
		"label":       "Am Wall 1, 30159 Hannover",
	}
	lat, lon := 52.3705, 9.7332
	rememberResolvedAddress(lat, lon, base)

	// ~3 m weiter — gleiche Adresse wiederverwenden.
	nearLat := lat + 3.0/111320.0
	got := cachedAddressNear(nearLat, lon)
	if got == nil || got["street"] != "Am Wall 1" {
		t.Fatalf("expected cache hit within 5 m, got=%v", got)
	}
	// Kopie: Mutation darf Cache nicht verändern.
	got["street"] = "mutated"
	again := cachedAddressNear(nearLat, lon)
	if again["street"] != "Am Wall 1" {
		t.Fatal("cache must return a copy")
	}

	// ~10 m weiter — neuer Lookup nötig.
	farLat := lat + 10.0/111320.0
	if cachedAddressNear(farLat, lon) != nil {
		t.Fatal("expected cache miss beyond 5 m")
	}
}
