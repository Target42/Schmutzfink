package records

import (
	"encoding/json"
	"fmt"
	"math"
)

type ROI struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

const roiMin = 0.01

func NormalizeROI(in ROI) (*ROI, error) {
	x1, x2 := in.X, in.X+in.W
	y1, y2 := in.Y, in.Y+in.H
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	x1, x2 = clamp01(x1), clamp01(x2)
	y1, y2 = clamp01(y1), clamp01(y2)
	w := x2 - x1
	h := y2 - y1
	if w < roiMin || h < roiMin {
		return nil, fmt.Errorf("Ausschnitt zu klein")
	}
	return &ROI{X: round6(x1), Y: round6(y1), W: round6(w), H: round6(h)}, nil
}

func ParseROIJSON(raw []byte) (*ROI, error) {
	var in ROI
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("Ausschnitt ungültig")
	}
	return NormalizeROI(in)
}

func ROIEqual(a, b *ROI) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	const eps = 1e-5
	return math.Abs(a.X-b.X) < eps && math.Abs(a.Y-b.Y) < eps &&
		math.Abs(a.W-b.W) < eps && math.Abs(a.H-b.H) < eps
}

func roiFromNulls(x, y, w, h *float64) *ROI {
	if x == nil || y == nil || w == nil || h == nil {
		return nil
	}
	return &ROI{X: *x, Y: *y, W: *w, H: *h}
}

func roiCols(r *ROI) (any, any, any, any) {
	if r == nil {
		return nil, nil, nil, nil
	}
	return r.X, r.Y, r.W, r.H
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}
