package records

import (
	"testing"
	"time"
)

func TestParseCaseKind(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "civil", want: KindCivil},
		{in: "Zivilrechtlich", want: KindCivil},
		{in: "criminal", want: KindCriminal},
		{in: "strafrecht", want: KindCriminal},
		{in: "", wantErr: true},
		{in: "other", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ParseCaseKind(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseCaseKind(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseCaseKind(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestRetentionYears(t *testing.T) {
	if got := RetentionYears(KindCivil); got != 3 {
		t.Fatalf("civil: %d", got)
	}
	if got := RetentionYears(KindCriminal); got != 5 {
		t.Fatalf("criminal: %d", got)
	}
}

func TestLocationDueAt(t *testing.T) {
	ref := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	if got := LocationDueAt(KindCivil, ref); got != time.Date(2027, 3, 15, 10, 0, 0, 0, time.UTC) {
		t.Fatalf("civil due: %s", got)
	}
	if got := LocationDueAt(KindCriminal, ref); got != time.Date(2029, 3, 15, 10, 0, 0, 0, time.UTC) {
		t.Fatalf("criminal due: %s", got)
	}
}

func TestRedactLocationExtraRemovesAddress(t *testing.T) {
	got := RedactLocationExtra(map[string]any{
		"address": map[string]any{"street": "Am Wall 1", "postal_code": "30159", "city": "Hannover"},
		"import":  map[string]any{"id": "wall_001"},
	})
	if _, ok := got["address"]; ok {
		t.Fatalf("address remains: %+v", got)
	}
	imp, _ := got["import"].(map[string]any)
	if imp["id"] != "wall_001" {
		t.Fatalf("import lost: %+v", got)
	}
}

func TestApplyCaseLocation(t *testing.T) {
	closed := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if applyCaseLocation(KindCivil, nil, nil) != nil {
		t.Fatal("open case should have no due date")
	}
	done := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	if applyCaseLocation(KindCivil, &done, &closed) != nil {
		t.Fatal("redacted photo should have no due date")
	}
	due := applyCaseLocation(KindCivil, nil, &closed)
	if due == nil || !due.Equal(time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("due: %v", due)
	}
}
