package records

import "testing"

func TestParseMinScore(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantNil bool
		wantErr bool
	}{
		{in: "", wantNil: true},
		{in: "  ", wantNil: true},
		{in: "0.5", want: 0.5},
		{in: "0", want: 0},
		{in: "1", want: 1},
		{in: "70", want: 0.7},
		{in: "30", want: 0.3},
		{in: "85", want: 0.85},
		{in: "101", wantErr: true},
		{in: "-0.1", wantErr: true},
		{in: "x", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ParseMinScore(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseMinScore(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMinScore(%q): %v", tc.in, err)
			continue
		}
		if tc.wantNil {
			if got != nil {
				t.Errorf("ParseMinScore(%q) = %v, want nil", tc.in, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			t.Errorf("ParseMinScore(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMinScoreOrDefault(t *testing.T) {
	if got := MinScoreOrDefault(nil); got != DefaultMinScore {
		t.Fatalf("nil: %v", got)
	}
	v := 0.7
	if got := MinScoreOrDefault(&v); got != 0.7 {
		t.Fatalf("set: %v", got)
	}
	low := -1.0
	if got := MinScoreOrDefault(&low); got != 0 {
		t.Fatalf("clamp: %v", got)
	}
}

func TestClampMinScore(t *testing.T) {
	if got := ClampMinScore(-2); got != 0 {
		t.Fatalf("low: %v", got)
	}
	if got := ClampMinScore(1.4); got != 1 {
		t.Fatalf("high: %v", got)
	}
	if got := ClampMinScore(0.3); got != 0.3 {
		t.Fatalf("mid: %v", got)
	}
}
