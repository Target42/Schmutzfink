package records

import (
	"context"
	"strings"
	"time"
)

// FollowUpAfter ist das Alter, ab dem ein Motiv vor Ort ein Kontrollfoto braucht.
const FollowUpAfter = 90 * 24 * time.Hour

type NearbyHit struct {
	SightingID  string
	RecordID    string
	Note        string
	Lat         float64
	Lon         float64
	SeenAt      time.Time
	MotifID     string
	MotifTitle  string
	MotifLastAt *time.Time
	CaseID      string
	CaseTitle   string
	DistanceM   float64
}

type NearbyPlace struct {
	Key        string    `json:"key"`
	MotifID    string    `json:"motif_id,omitempty"`
	Title      string    `json:"title"`
	CaseID     string    `json:"case_id,omitempty"`
	CaseTitle  string    `json:"case_title,omitempty"`
	LastAt     time.Time `json:"last_at"`
	DistanceM  float64   `json:"distance_m"`
	Count      int       `json:"count"`
	SightingID string    `json:"sighting_id"`
	ThumbURL   string    `json:"thumb_url,omitempty"`
	FollowUp   bool      `json:"follow_up"`
}

func (r *Repo) NearbyHits(ctx context.Context, tenantID string, lat, lon, radiusM float64, limit int) ([]NearbyHit, error) {
	if limit <= 0 || limit > 80 {
		limit = 40
	}
	minLat, maxLat, minLon, maxLon := geoBBox(lat, lon, radiusM)
	rows, err := r.DB.Query(ctx, `
		SELECT sighting_id, record_id, note, lat, lon, seen_at,
		       motif_id, motif_title, motif_last_at, case_id, case_title, distance_m
		FROM (
			SELECT DISTINCT ON (r.id)
				s.id::text AS sighting_id,
				r.id::text AS record_id,
				s.note,
				r.lat, r.lon,
				COALESCE(r.captured_at, r.uploaded_at) AS seen_at,
				COALESCE(s.motif_id::text, '') AS motif_id,
				COALESCE(m.title, '') AS motif_title,
				(
					SELECT max(COALESCE(r2.captured_at, r2.uploaded_at))
					FROM sightings s2
					JOIN records r2 ON r2.id = s2.record_id
					WHERE s2.motif_id = s.motif_id
				) AS motif_last_at,
				COALESCE(r.case_id::text, '') AS case_id,
				COALESCE(c.title, '') AS case_title,
				(2 * 6371000 * asin(sqrt(LEAST(1.0,
					power(sin(radians(r.lat - $6)/2), 2) +
					cos(radians($6)) * cos(radians(r.lat)) *
					power(sin(radians(r.lon - $7)/2), 2)
				)))) AS distance_m
			FROM sightings s
			JOIN records r ON r.id = s.record_id
			LEFT JOIN motifs m ON m.id = s.motif_id
			LEFT JOIN cases c ON c.id = r.case_id
			WHERE r.tenant_id = $1
			  AND r.lat IS NOT NULL AND r.lon IS NOT NULL
			  AND r.lat BETWEEN $2 AND $3
			  AND r.lon BETWEEN $4 AND $5
			  AND (2 * 6371000 * asin(sqrt(LEAST(1.0,
					power(sin(radians(r.lat - $6)/2), 2) +
					cos(radians($6)) * cos(radians(r.lat)) *
					power(sin(radians(r.lon - $7)/2), 2)
			  )))) <= $8
			ORDER BY r.id, (s.motif_id IS NULL), s.id
		) hits
		ORDER BY distance_m
		LIMIT $9`,
		tenantID, minLat, maxLat, minLon, maxLon, lat, lon, radiusM, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NearbyHit
	for rows.Next() {
		var h NearbyHit
		if err := rows.Scan(
			&h.SightingID, &h.RecordID, &h.Note, &h.Lat, &h.Lon, &h.SeenAt,
			&h.MotifID, &h.MotifTitle, &h.MotifLastAt, &h.CaseID, &h.CaseTitle, &h.DistanceM,
		); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if out == nil {
		out = []NearbyHit{}
	}
	return out, rows.Err()
}

// GroupNearby fasst Fotos desselben Motivs zusammen. Ohne Motiv bleibt jedes Foto einzeln.
// Follow-up gilt für das letzte Foto des Motivs, nicht nur für den Treffer im Umkreis.
func GroupNearby(hits []NearbyHit, now time.Time) []NearbyPlace {
	order := make([]string, 0, len(hits))
	by := map[string]*NearbyPlace{}
	for _, h := range hits {
		key := "record:" + h.RecordID
		title := strings.TrimSpace(h.Note)
		if title == "" {
			title = "Ohne Motiv"
		}
		last := h.SeenAt
		if h.MotifID != "" {
			key = "motif:" + h.MotifID
			if t := strings.TrimSpace(h.MotifTitle); t != "" {
				title = t
			} else {
				title = "Motiv"
			}
			if h.MotifLastAt != nil && !h.MotifLastAt.IsZero() {
				last = *h.MotifLastAt
			}
		}
		cur, ok := by[key]
		if !ok {
			p := NearbyPlace{
				Key:        key,
				MotifID:    h.MotifID,
				Title:      title,
				CaseID:     h.CaseID,
				CaseTitle:  h.CaseTitle,
				LastAt:     last,
				DistanceM:  h.DistanceM,
				Count:      1,
				SightingID: h.SightingID,
			}
			by[key] = &p
			order = append(order, key)
			continue
		}
		cur.Count++
		if h.DistanceM < cur.DistanceM {
			cur.DistanceM = h.DistanceM
			cur.SightingID = h.SightingID
			if h.CaseID != "" {
				cur.CaseID = h.CaseID
				cur.CaseTitle = h.CaseTitle
			}
		} else if cur.CaseID == "" && h.CaseID != "" {
			cur.CaseID = h.CaseID
			cur.CaseTitle = h.CaseTitle
		}
		if h.MotifID == "" && h.SeenAt.After(cur.LastAt) {
			cur.LastAt = h.SeenAt
		}
		if h.MotifLastAt != nil && h.MotifLastAt.After(cur.LastAt) {
			cur.LastAt = *h.MotifLastAt
		}
	}
	out := make([]NearbyPlace, 0, len(order))
	for _, key := range order {
		p := *by[key]
		if p.MotifID != "" && !p.LastAt.IsZero() && !now.Before(p.LastAt.Add(FollowUpAfter)) {
			p.FollowUp = true
		}
		out = append(out, p)
	}
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && nearbyLess(out[j], out[j-1]) {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func nearbyLess(a, b NearbyPlace) bool {
	if a.FollowUp != b.FollowUp {
		return a.FollowUp
	}
	if a.DistanceM != b.DistanceM {
		return a.DistanceM < b.DistanceM
	}
	return a.Title < b.Title
}
