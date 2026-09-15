package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/records"
)

type sightingBody struct {
	Note   *string         `json:"note"`
	Tags   *[]string       `json:"tags"`
	ROI    json.RawMessage `json:"roi"`
	Fields json.RawMessage `json:"fields"`
}

func (s *Server) addSighting(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	recordID := chi.URLParam(r, "id")
	if _, err := s.Records.GetPhoto(r.Context(), p.TenantID, recordID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
			return
		}
		writeError(w, http.StatusInternalServerError, "Datensatz konnte nicht geladen werden")
		return
	}
	var body sightingBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	in := records.Sighting{ID: uuid.NewString()}
	if body.Note != nil {
		in.Note = *body.Note
	}
	if body.Tags != nil {
		in.Tags = *body.Tags
	}
	roi, err := parseOptionalROI(body.ROI, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if roi == nil {
		writeError(w, http.StatusBadRequest, "Bitte ein Graffiti umrahmen")
		return
	}
	in.ROI = roi
	created, err := s.Records.AddSighting(r.Context(), p.TenantID, recordID, in)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Sichtung konnte nicht angelegt werden")
		return
	}
	if !applySightingFields(w, r, s.Records, p.TenantID, created.ID, body.Fields) {
		return
	}
	if s.Jobs != nil {
		s.Jobs.Notify()
	}
	s.writeAudit(r, p, audit.SightingAdd, created.ID, map[string]any{"record_id": recordID})
	photo, err := s.Records.GetPhoto(r.Context(), p.TenantID, recordID)
	if err != nil {
		writeJSON(w, http.StatusCreated, presentNestedSighting(created))
		return
	}
	writeJSON(w, http.StatusCreated, presentPhoto(photo))
}

func (s *Server) patchSighting(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	cur, err := s.Records.GetSighting(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Sichtung nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Sichtung konnte nicht geladen werden")
		return
	}
	var body sightingBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	in := records.Sighting{
		ID:       cur.ID,
		RecordID: cur.RecordID,
		Note:     cur.Note,
		Tags:     cur.Tags,
		ROI:      cur.ROI,
	}
	if body.Note != nil {
		in.Note = *body.Note
	}
	if body.Tags != nil {
		in.Tags = *body.Tags
	}
	if len(body.ROI) > 0 {
		roi, err := parseOptionalROI(body.ROI, false)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		in.ROI = roi
	}
	oldROI := cur.ROI
	if _, err := s.Records.UpdateSighting(r.Context(), p.TenantID, id, in); err != nil {
		writeError(w, http.StatusInternalServerError, "Sichtung konnte nicht gespeichert werden")
		return
	}
	if !applySightingFields(w, r, s.Records, p.TenantID, id, body.Fields) {
		return
	}
	if !records.ROIEqual(oldROI, in.ROI) {
		if err := s.Records.RequeueEmbedding(r.Context(), p.TenantID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "Ausschnitt konnte nicht übernommen werden")
			return
		}
		if s.Jobs != nil {
			s.Jobs.Notify()
		}
	}
	s.writeAudit(r, p, audit.SightingPatch, id, map[string]any{"record_id": cur.RecordID})
	photo, err := s.Records.GetPhoto(r.Context(), p.TenantID, cur.RecordID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datensatz konnte nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, presentPhoto(photo))
}

func (s *Server) deleteSighting(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	photo, err := s.Records.DeleteSighting(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Sichtung nicht gefunden")
		return
	}
	if errors.Is(err, records.ErrLastSighting) {
		writeError(w, http.StatusBadRequest, "Die letzte Sichtung kann nicht gelöscht werden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Sichtung konnte nicht gelöscht werden")
		return
	}
	if s.Jobs != nil {
		s.Jobs.Notify()
	}
	s.writeAudit(r, p, audit.SightingDelete, id, map[string]any{"record_id": photo.ID})
	writeJSON(w, http.StatusOK, presentPhoto(photo))
}

func (s *Server) similarSightings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := s.Records.GetSighting(r.Context(), principal(r).TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Sichtung nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Sichtung konnte nicht geladen werden")
		return
	}
	if rec.EmbeddingStatus != "ready" {
		writeJSON(w, http.StatusOK, map[string]any{
			"items":  []map[string]any{},
			"status": rec.EmbeddingStatus,
		})
		return
	}
	if err := s.requireEmbeddings(w); err != nil {
		return
	}
	minScore, err := queryMinScore(r, records.DefaultMinScore)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := s.Records.Similar(r.Context(), principal(r).TenantID, id, 12, minScore)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"items": []map[string]any{}, "status": "pending"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Ähnliche Bilder konnten nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": presentRecords(items), "status": "ready"})
}

func (s *Server) serveSightingThumb(w http.ResponseWriter, r *http.Request) {
	rec, err := s.Records.GetSighting(r.Context(), principal(r).TenantID, chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Sichtung nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datei konnte nicht geladen werden")
		return
	}
	if rec.ROI == nil {
		id := rec.ThumbObjectID
		ct := rec.ContentType
		if rec.ThumbObjectID != rec.OriginalObjectID {
			ct = "image/jpeg"
		}
		rc, err := s.Store.Open(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "Datei nicht gefunden")
			return
		}
		defer rc.Close()
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "private, max-age=120")
		_, _ = io.Copy(w, rc)
		return
	}
	rc, err := s.Store.Open(r.Context(), rec.OriginalObjectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Datei nicht gefunden")
		return
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datei konnte nicht geladen werden")
		return
	}
	thumb, err := s.croppedThumb(rec.ID, rec.ROI, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorschaubild konnte nicht erzeugt werden")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=120")
	_, _ = w.Write(thumb)
}

func parseOptionalROI(raw json.RawMessage, required bool) (*records.ROI, error) {
	if len(raw) == 0 {
		if required {
			return nil, errors.New("Bitte ein Graffiti umrahmen")
		}
		return nil, nil
	}
	if string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	return records.ParseROIJSON(raw)
}
