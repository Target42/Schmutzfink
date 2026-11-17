package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/db"
	"schmutzfink/internal/records"
)

func (s *Server) publicConfig(w http.ResponseWriter, r *http.Request) {
	publicNoIndex(w)
	writeJSON(w, http.StatusOK, map[string]any{"public_map": s.publicMapOn(r)})
}

func (s *Server) publicMap(w http.ResponseWriter, r *http.Request) {
	publicNoIndex(w)
	if !s.publicMapOn(r) {
		writeError(w, http.StatusNotFound, "Nicht gefunden")
		return
	}
	items, err := s.Records.MapPoints(r.Context(), db.DefaultTenantID, records.ListFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Kartenpunkte konnten nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": presentPublicPoints(items)})
}

func (s *Server) publicSightingThumb(w http.ResponseWriter, r *http.Request) {
	publicNoIndex(w)
	if !s.publicMapOn(r) {
		writeError(w, http.StatusNotFound, "Nicht gefunden")
		return
	}
	rec, err := s.Records.GetSighting(r.Context(), db.DefaultTenantID, chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datei konnte nicht geladen werden")
		return
	}
	if rec.Lat == nil || rec.Lon == nil {
		writeError(w, http.StatusNotFound, "Nicht gefunden")
		return
	}
	if rec.ROI == nil && rec.ThumbObjectID != "" && rec.ThumbObjectID != rec.OriginalObjectID {
		s.writeSightingThumb(w, r, rec)
		return
	}
	s.writeDerivedPublicThumb(w, r, rec)
}

func (s *Server) writeDerivedPublicThumb(w http.ResponseWriter, r *http.Request, rec records.Record) {
	if s.Store == nil {
		writeError(w, http.StatusNotFound, "Datei nicht gefunden")
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

func (s *Server) publicMapOn(r *http.Request) bool {
	if s.Records == nil || s.Records.DB == nil {
		return false
	}
	on, err := s.Records.PublicMap(r.Context(), db.DefaultTenantID)
	return err == nil && on
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	if s.Records == nil {
		writeError(w, http.StatusInternalServerError, "Einstellungen konnten nicht geladen werden")
		return
	}
	on, err := s.Records.PublicMap(r.Context(), principal(r).TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Einstellungen konnten nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"public_map": on})
}

func (s *Server) patchSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicMap *bool `json:"public_map"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PublicMap == nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if s.Records == nil {
		writeError(w, http.StatusInternalServerError, "Einstellungen konnten nicht gespeichert werden")
		return
	}
	p := principal(r)
	if err := s.Records.SetPublicMap(r.Context(), p.TenantID, *body.PublicMap); err != nil {
		writeError(w, http.StatusInternalServerError, "Einstellungen konnten nicht gespeichert werden")
		return
	}
	s.writeAudit(r, p, audit.SettingsPatch, "public_map", map[string]any{"public_map": *body.PublicMap})
	writeJSON(w, http.StatusOK, map[string]any{"public_map": *body.PublicMap})
}

func presentPublicPoints(items []records.Record) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, rec := range items {
		if rec.Lat == nil || rec.Lon == nil {
			continue
		}
		out = append(out, map[string]any{
			"id":        rec.ID,
			"lat":       rec.Lat,
			"lon":       rec.Lon,
			"thumb_url": publicThumbURL(rec.ID, rec.ROI),
		})
	}
	return out
}

func publicThumbURL(id string, roi *records.ROI) string {
	u := "/api/public/sightings/" + id + "/thumb"
	if roi == nil {
		return u
	}
	return fmt.Sprintf("%s?x=%.4f&y=%.4f&w=%.4f&h=%.4f", u, roi.X, roi.Y, roi.W, roi.H)
}

func publicNoIndex(w http.ResponseWriter) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
}
