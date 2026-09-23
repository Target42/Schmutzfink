package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/ingest"
	"schmutzfink/internal/records"
)

func (s *Server) listRecords(w http.ResponseWriter, r *http.Request) {
	s.listWithOptionalSemantic(w, r, false)
}

func (s *Server) mapRecords(w http.ResponseWriter, r *http.Request) {
	s.listWithOptionalSemantic(w, r, true)
}

func (s *Server) getRecord(w http.ResponseWriter, r *http.Request) {
	photo, err := s.Records.GetPhoto(r.Context(), principal(r).TenantID, chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datensatz konnte nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, presentPhoto(photo))
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, ingest.MaxBytes+1<<20)
	if err := r.ParseMultipartForm(ingest.MaxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "Datei zu groß oder ungültig")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Datei fehlt")
		return
	}
	defer file.Close()

	data, err := ingest.ReadLimited(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Datei zu groß")
		return
	}

	in := photoInput{
		Note: strings.TrimSpace(r.FormValue("note")),
		Tags: parseTags(r.FormValue("tags")),
	}
	if raw := strings.TrimSpace(r.FormValue("fields")); raw != "" {
		fields, err := parseFieldValues(json.RawMessage(raw))
		if err != nil {
			writeError(w, http.StatusBadRequest, "Felder ungültig")
			return
		}
		in.Fields = fields
	}
	if strings.EqualFold(strings.TrimSpace(r.FormValue("clear_gps")), "true") {
		in.ClearGPS = true
	} else {
		if lat := strings.TrimSpace(r.FormValue("lat")); lat != "" {
			if v, err := strconv.ParseFloat(lat, 64); err == nil {
				in.Lat = &v
			}
		}
		if lon := strings.TrimSpace(r.FormValue("lon")); lon != "" {
			if v, err := strconv.ParseFloat(lon, 64); err == nil {
				in.Lon = &v
			}
		}
		if (in.Lat == nil) != (in.Lon == nil) {
			writeError(w, http.StatusBadRequest, "Breite und Länge gehören zusammen")
			return
		}
	}
	if raw := strings.TrimSpace(r.FormValue("captured_at")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			in.CapturedAt = &t
		}
	}
	if cid := strings.TrimSpace(r.FormValue("case_id")); cid != "" {
		if _, err := uuid.Parse(cid); err != nil {
			writeError(w, http.StatusBadRequest, "Vorgang ungültig")
			return
		}
		in.CaseID = cid
	}
	if raw := strings.TrimSpace(r.FormValue("roi")); raw != "" {
		roi, err := records.ParseROIJSON([]byte(raw))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		in.ROI = roi
	}
	if addr, ok := parseAddressJSON(r.FormValue("address")); ok {
		in.Extra = setExtraAddress(in.Extra, addr)
	}

	stored, err := s.savePhoto(r.Context(), principal(r), data, in)
	if err != nil {
		var dup *duplicateError
		if errors.As(err, &dup) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":     dup.Error(),
				"record_id": dup.Photo.ID,
				"record":    presentPhoto(dup.Photo),
			})
			return
		}
		if errors.Is(err, ingest.ErrTooManyPixels) {
			writeError(w, http.StatusBadRequest, "Bildauflösung zu hoch")
			return
		}
		if errors.Is(err, records.ErrInvalidFieldValue) || errors.Is(err, records.ErrFieldKey) {
			writeError(w, http.StatusBadRequest, "Feldwert ungültig oder Pflichtfeld leer")
			return
		}
		if errors.Is(err, records.ErrUnknownCase) {
			writeError(w, http.StatusBadRequest, "Vorgang nicht gefunden")
			return
		}
		if strings.Contains(err.Error(), "unsupported type") {
			writeError(w, http.StatusBadRequest, "Nur JPEG, PNG, GIF oder WebP")
			return
		}
		writeError(w, http.StatusInternalServerError, "Datensatz konnte nicht angelegt werden")
		return
	}
	if s.Jobs != nil {
		s.Jobs.Notify()
	}
	s.writeAudit(r, principal(r), audit.RecordUpload, stored.ID, nil)
	writeJSON(w, http.StatusCreated, presentPhoto(stored))
}

type patchPhotoBody struct {
	Lat          *float64    `json:"lat"`
	Lon          *float64    `json:"lon"`
	ClearGPS     bool        `json:"clear_gps"`
	CapturedAt   *time.Time  `json:"captured_at"`
	Address      *geoAddress `json:"address"`
	ClearAddress bool        `json:"clear_address"`
}

func (s *Server) patchRecord(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	photo, err := s.Records.GetPhoto(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datensatz konnte nicht geladen werden")
		return
	}
	var body patchPhotoBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if body.ClearGPS {
		photo.Lat = nil
		photo.Lon = nil
		photo.Extra = setExtraAddress(photo.Extra, nil)
	} else {
		if body.Lat != nil {
			photo.Lat = body.Lat
		}
		if body.Lon != nil {
			photo.Lon = body.Lon
		}
	}
	if body.ClearAddress {
		photo.Extra = setExtraAddress(photo.Extra, nil)
	} else if body.Address != nil {
		if raw, err := json.Marshal(body.Address); err == nil {
			if addr, ok := parseAddressJSON(string(raw)); ok {
				photo.Extra = setExtraAddress(photo.Extra, addr)
				if photo.Lat != nil && photo.Lon != nil {
					rememberResolvedAddress(*photo.Lat, *photo.Lon, addr)
				}
			}
		}
	} else if !body.ClearGPS && photo.Lat != nil && photo.Lon != nil && addressFromExtra(photo.Extra) == nil {
		if addr := lookupAddress(r.Context(), *photo.Lat, *photo.Lon); addr != nil {
			photo.Extra = setExtraAddress(photo.Extra, addr)
		}
	}
	if body.CapturedAt != nil {
		photo.CapturedAt = body.CapturedAt
	}
	updated, err := s.Records.UpdatePhoto(r.Context(), p.TenantID, id, photo)
	if errors.Is(err, records.ErrLocationRedacted) {
		writeError(w, http.StatusConflict, "Standort wurde nach Ablauf der Frist entfernt")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datensatz konnte nicht gespeichert werden")
		return
	}
	s.writeAudit(r, p, audit.RecordPatch, id, nil)
	writeJSON(w, http.StatusOK, presentPhoto(updated))
}

func (s *Server) serveOriginal(w http.ResponseWriter, r *http.Request) {
	s.servePhotoObject(w, r, false)
}

func (s *Server) serveThumb(w http.ResponseWriter, r *http.Request) {
	s.servePhotoObject(w, r, true)
}

func (s *Server) servePhotoObject(w http.ResponseWriter, r *http.Request, thumb bool) {
	photo, err := s.Records.GetPhoto(r.Context(), principal(r).TenantID, chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datei konnte nicht geladen werden")
		return
	}
	id := photo.OriginalObjectID
	ct := photo.ContentType
	if thumb {
		id = photo.ThumbObjectID
		if photo.ThumbObjectID != photo.OriginalObjectID {
			ct = "image/jpeg"
		}
	}
	rc, err := s.Store.Open(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Datei nicht gefunden")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, rc)
}

func parseFilter(r *http.Request) (records.ListFilter, bool, error) {
	q := r.URL.Query()
	f := records.ListFilter{Query: strings.TrimSpace(q.Get("q"))}
	semantic := q.Get("semantic") == "true" || q.Get("semantic") == "1"
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, false, errors.New("limit ungültig")
		}
		f.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, false, errors.New("offset ungültig")
		}
		f.Offset = n
	}
	if v := q.Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return f, false, errors.New("from muss YYYY-MM-DD sein")
		}
		f.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return f, false, errors.New("to muss YYYY-MM-DD sein")
		}
		t = t.Add(24 * time.Hour)
		f.To = &t
	}
	switch q.Get("has_gps") {
	case "":
	case "true", "1":
		v := true
		f.HasGPS = &v
	case "false", "0":
		v := false
		f.HasGPS = &v
	default:
		return f, false, errors.New("has_gps muss true oder false sein")
	}
	switch q.Get("edited") {
	case "":
	case "true", "1":
		v := true
		f.Edited = &v
	case "false", "0":
		v := false
		f.Edited = &v
	default:
		return f, false, errors.New("edited muss true oder false sein")
	}

	nearLat, nearLon, radiusM := q.Get("near_lat"), q.Get("near_lon"), q.Get("radius_m")
	if nearLat != "" || nearLon != "" || radiusM != "" {
		if nearLat == "" || nearLon == "" || radiusM == "" {
			return f, false, errors.New("Umkreis braucht near_lat, near_lon und radius_m")
		}
		lat, err := strconv.ParseFloat(nearLat, 64)
		if err != nil || lat < -90 || lat > 90 {
			return f, false, errors.New("near_lat ungültig")
		}
		lon, err := strconv.ParseFloat(nearLon, 64)
		if err != nil || lon < -180 || lon > 180 {
			return f, false, errors.New("near_lon ungültig")
		}
		radius, err := strconv.ParseFloat(radiusM, 64)
		if err != nil || radius < 10 || radius > 50000 {
			return f, false, errors.New("radius_m muss zwischen 10 und 50000 liegen")
		}
		f.NearLat = &lat
		f.NearLon = &lon
		f.RadiusM = &radius
	}
	for key, vals := range q {
		if !strings.HasPrefix(key, "cf.") || len(vals) == 0 {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(key, "cf."))
		val := strings.TrimSpace(vals[0])
		if name == "" || val == "" {
			continue
		}
		f.FieldFilters = append(f.FieldFilters, records.FieldFilter{Key: name, Value: val})
	}
	if mid := strings.TrimSpace(q.Get("motif_id")); mid != "" {
		if mid != "none" {
			if _, err := uuid.Parse(mid); err != nil {
				return f, false, errors.New("motif_id ungültig")
			}
		}
		f.MotifID = mid
	}
	if cid := strings.TrimSpace(q.Get("case_id")); cid != "" {
		if cid != "none" {
			if _, err := uuid.Parse(cid); err != nil {
				return f, false, errors.New("case_id ungültig")
			}
		}
		f.CaseID = cid
	}
	score, err := records.ParseMinScore(q.Get("min_score"))
	if err != nil {
		return f, false, err
	}
	f.MinScore = score
	return f, semantic, nil
}

func queryMinScore(r *http.Request, fallback float64) (float64, error) {
	p, err := records.ParseMinScore(r.URL.Query().Get("min_score"))
	if err != nil {
		return 0, err
	}
	if p == nil {
		return fallback, nil
	}
	return *p, nil
}

func parseTags(raw string) []string {
	var tags []string
	for _, part := range strings.Split(raw, ",") {
		t := strings.TrimSpace(part)
		if t != "" {
			tags = append(tags, t)
		}
	}
	if tags == nil {
		tags = []string{}
	}
	return tags
}

func presentRecords(items []records.Record) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, rec := range items {
		out = append(out, presentSightingHit(rec))
	}
	return out
}

func presentSightingHit(rec records.Record) map[string]any {
	out := map[string]any{
		"id":                   rec.ID,
		"record_id":            rec.RecordID,
		"note":                 rec.Note,
		"tags":                 rec.Tags,
		"lat":                  rec.Lat,
		"lon":                  rec.Lon,
		"captured_at":          rec.CapturedAt,
		"uploaded_at":          rec.UploadedAt,
		"uploaded_by":          rec.UploadedBy,
		"uploaded_by_name":     rec.UploadedByName,
		"content_type":         rec.ContentType,
		"embedding_status":     rec.EmbeddingStatus,
		"has_gps":              rec.Lat != nil && rec.Lon != nil,
		"thumb_url":            sightingThumbURL(rec.ID, rec.ROI),
		"original_url":         "/api/records/" + rec.RecordID + "/original",
		"roi":                  rec.ROI,
		"motif_id":             rec.MotifID,
		"motif_title":          rec.MotifTitle,
		"case_id":              rec.CaseID,
		"case_title":           rec.CaseTitle,
		"case_kind":            rec.CaseKind,
		"case_closed_at":       rec.CaseClosedAt,
		"location_redacted_at": rec.LocationRedactedAt,
		"location_due_at":      rec.LocationDueAt,
		"fields":               rec.Fields,
	}
	if rec.Score != nil {
		out["score"] = rec.Score
	}
	if addr := addressFromExtra(rec.Extra); addr != nil {
		out["address"] = addr
	}
	return out
}

func presentPhoto(photo records.Photo) map[string]any {
	sightings := make([]map[string]any, 0, len(photo.Sightings))
	for _, s := range photo.Sightings {
		sightings = append(sightings, presentNestedSighting(s))
	}
	out := map[string]any{
		"id":                   photo.ID,
		"lat":                  photo.Lat,
		"lon":                  photo.Lon,
		"captured_at":          photo.CapturedAt,
		"uploaded_at":          photo.UploadedAt,
		"uploaded_by":          photo.UploadedBy,
		"uploaded_by_name":     photo.UploadedByName,
		"content_type":         photo.ContentType,
		"has_gps":              photo.Lat != nil && photo.Lon != nil,
		"thumb_url":            "/api/records/" + photo.ID + "/thumb",
		"original_url":         "/api/records/" + photo.ID + "/original",
		"sightings":            sightings,
		"deletion_request":     presentDeletion(photo.Deletion),
		"case_id":              photo.CaseID,
		"case_title":           photo.CaseTitle,
		"case_kind":            photo.CaseKind,
		"case_closed_at":       photo.CaseClosedAt,
		"location_redacted_at": photo.LocationRedactedAt,
		"location_due_at":      photo.LocationDueAt,
	}
	if addr := addressFromExtra(photo.Extra); addr != nil {
		out["address"] = addr
	}
	return out
}

func presentDeletion(d *records.DeletionRequest) any {
	if d == nil {
		return nil
	}
	return map[string]any{
		"requested_at":      d.RequestedAt,
		"requested_by":      d.RequestedBy,
		"requested_by_name": d.RequestedByName,
		"reason":            d.Reason,
	}
}

func presentNestedSighting(s records.Sighting) map[string]any {
	return map[string]any{
		"id":               s.ID,
		"record_id":        s.RecordID,
		"note":             s.Note,
		"tags":             s.Tags,
		"roi":              s.ROI,
		"embedding_status": s.EmbeddingStatus,
		"thumb_url":        sightingThumbURL(s.ID, s.ROI),
		"motif_id":         s.MotifID,
		"motif_title":      s.MotifTitle,
		"fields":           s.Fields,
	}
}

func sightingThumbURL(id string, roi *records.ROI) string {
	u := "/api/sightings/" + id + "/thumb"
	if roi == nil {
		return u
	}
	return fmt.Sprintf("%s?x=%.4f&y=%.4f&w=%.4f&h=%.4f", u, roi.X, roi.Y, roi.W, roi.H)
}
