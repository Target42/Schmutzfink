package records

import "testing"

func TestNormalizeROI(t *testing.T) {
	got, err := NormalizeROI(ROI{X: 0.6, Y: 0.7, W: -0.4, H: -0.2})
	if err != nil {
		t.Fatal(err)
	}
	if got.X != 0.2 || got.Y != 0.5 || got.W != 0.4 || got.H != 0.2 {
		t.Fatalf("got %+v", got)
	}
}

func TestNormalizeROITooSmall(t *testing.T) {
	if _, err := NormalizeROI(ROI{X: 0.1, Y: 0.1, W: 0.001, H: 0.5}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseROIJSON(t *testing.T) {
	got, err := ParseROIJSON([]byte(`{"x":0.1,"y":0.2,"w":0.3,"h":0.4}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.X != 0.1 || got.Y != 0.2 || got.W != 0.3 || got.H != 0.4 {
		t.Fatalf("got %+v", got)
	}
}
