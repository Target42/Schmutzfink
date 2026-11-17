package records

import (
	"errors"
	"strconv"
	"strings"
)

const DefaultMinScore = 0.5

func ParseMinScore(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, errors.New("min_score ungültig")
	}
	if v > 1 && v <= 100 {
		v /= 100
	}
	if v < 0 || v > 1 {
		return nil, errors.New("min_score muss zwischen 0 und 1 liegen")
	}
	return &v, nil
}

func MinScoreOrDefault(p *float64) float64 {
	if p == nil {
		return DefaultMinScore
	}
	return ClampMinScore(*p)
}

func ClampMinScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
