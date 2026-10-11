package records

import "testing"

func TestFieldKeyFrom(t *testing.T) {
	if got := FieldKeyFrom("Aktenzeichen"); got != "aktenzeichen" {
		t.Fatalf("got %q", got)
	}
	if got := FieldKeyFrom("  Bezirk / Stadt  "); got != "bezirk_stadt" {
		t.Fatalf("got %q", got)
	}
	if got := FieldKeyFrom("Übermalung"); got != "uebermalung" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeFieldValue(t *testing.T) {
	if got, err := NormalizeFieldValue(FieldDef{Type: FieldBool}, "yes"); err != nil || got != "ja" {
		t.Fatalf("bool %q %v", got, err)
	}
	if _, err := NormalizeFieldValue(FieldDef{Type: FieldNumber}, "x"); err == nil {
		t.Fatal("expected number error")
	}
	if got, err := NormalizeFieldValue(FieldDef{Type: FieldSelect, Options: []string{"offen", "erledigt"}}, "Offen"); err != nil || got != "offen" {
		t.Fatalf("select %q %v", got, err)
	}
}
