package records

import (
	"math"
	"sort"
	"strings"
)

const summaryTop = 20

type NamedCount struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type Hotspot struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Count int     `json:"count"`
}

type FieldCounts struct {
	Key    string       `json:"key"`
	Label  string       `json:"label"`
	Values []NamedCount `json:"values"`
}

type Summary struct {
	Sightings  int           `json:"sightings"`
	Photos     int           `json:"photos"`
	WithGPS    int           `json:"with_gps"`
	WithoutGPS int           `json:"without_gps"`
	WithMotif  int           `json:"with_motif"`
	Total      int           `json:"total"`
	Truncated  bool          `json:"truncated"`
	Months     []NamedCount  `json:"months"`
	Tags       []NamedCount  `json:"tags"`
	Motifs     []NamedCount  `json:"motifs"`
	Fields     []FieldCounts `json:"fields"`
	Hotspots   []Hotspot     `json:"hotspots"`
}

func Summarize(items []Record, total int, defs []FieldDef) Summary {
	if items == nil {
		items = []Record{}
	}
	photos := map[string]struct{}{}
	months := map[string]int{}
	tags := map[string]int{}
	motifs := map[string]int{}
	motifLabel := map[string]string{}
	hot := map[[2]int]int{}
	fieldVals := map[string]map[string]int{}
	var withGPS, withMotif int

	for _, rec := range items {
		photos[rec.RecordID] = struct{}{}
		if rec.Lat != nil && rec.Lon != nil {
			withGPS++
			k := [2]int{gridKey(*rec.Lat), gridKey(*rec.Lon)}
			hot[k]++
		}
		when := rec.UploadedAt
		if rec.CapturedAt != nil {
			when = *rec.CapturedAt
		}
		months[when.UTC().Format("2006-01")]++
		for _, tag := range rec.Tags {
			if tag = strings.TrimSpace(tag); tag != "" {
				tags[tag]++
			}
		}
		if rec.MotifID != "" {
			withMotif++
			motifs[rec.MotifID]++
			if rec.MotifTitle != "" {
				motifLabel[rec.MotifID] = rec.MotifTitle
			}
		} else {
			motifs[""]++
			motifLabel[""] = "ohne Motiv"
		}
		for key, val := range rec.Fields {
			if val == "" {
				continue
			}
			if fieldVals[key] == nil {
				fieldVals[key] = map[string]int{}
			}
			fieldVals[key][val]++
		}
	}

	out := Summary{
		Sightings:  len(items),
		Photos:     len(photos),
		WithGPS:    withGPS,
		WithoutGPS: len(items) - withGPS,
		WithMotif:  withMotif,
		Total:      total,
		Truncated:  total > len(items),
		Months:     namedFromMapChrono(months),
		Tags:       topNamed(tags, nil, summaryTop),
		Motifs:     topNamed(motifs, motifLabel, summaryTop),
		Fields:     fieldCountsFrom(defs, fieldVals),
		Hotspots:   topHotspots(hot, summaryTop),
	}
	if out.Months == nil {
		out.Months = []NamedCount{}
	}
	if out.Tags == nil {
		out.Tags = []NamedCount{}
	}
	if out.Motifs == nil {
		out.Motifs = []NamedCount{}
	}
	if out.Fields == nil {
		out.Fields = []FieldCounts{}
	}
	if out.Hotspots == nil {
		out.Hotspots = []Hotspot{}
	}
	return out
}

func gridKey(v float64) int {
	return int(math.Round(v * 1000))
}

func namedFromMapChrono(m map[string]int) []NamedCount {
	out := make([]NamedCount, 0, len(m))
	for k, n := range m {
		out = append(out, NamedCount{Key: k, Label: k, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func topNamed(m map[string]int, labels map[string]string, n int) []NamedCount {
	out := make([]NamedCount, 0, len(m))
	for k, c := range m {
		label := k
		if labels != nil {
			if l, ok := labels[k]; ok && l != "" {
				label = l
			}
		}
		if label == "" {
			label = k
		}
		out = append(out, NamedCount{Key: k, Label: label, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func fieldCountsFrom(defs []FieldDef, vals map[string]map[string]int) []FieldCounts {
	out := make([]FieldCounts, 0, len(defs))
	for _, d := range defs {
		counts := vals[d.Key]
		if len(counts) == 0 {
			continue
		}
		out = append(out, FieldCounts{
			Key:    d.Key,
			Label:  d.Label,
			Values: topNamed(counts, nil, summaryTop),
		})
	}
	return out
}

func topHotspots(m map[[2]int]int, n int) []Hotspot {
	out := make([]Hotspot, 0, len(m))
	for k, c := range m {
		out = append(out, Hotspot{
			Lat:   float64(k[0]) / 1000,
			Lon:   float64(k[1]) / 1000,
			Count: c,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Lat != out[j].Lat {
			return out[i].Lat < out[j].Lat
		}
		return out[i].Lon < out[j].Lon
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}
