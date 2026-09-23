package records

import (
	"strings"
	"testing"
)

func TestMotifTitleFrom(t *testing.T) {
	if got := MotifTitleFrom("  "); got != "Motiv" {
		t.Fatalf("empty: %q", got)
	}
	if got := MotifTitleFrom("  Schriftzug  "); got != "Schriftzug" {
		t.Fatalf("trim: %q", got)
	}
	got := MotifTitleFrom(strings.Repeat("x", 90))
	if got != strings.Repeat("x", 80) {
		t.Fatalf("truncate: %q", got)
	}
}
