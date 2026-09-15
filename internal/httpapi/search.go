package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"schmutzfink/internal/clip"
	"schmutzfink/internal/ingest"
	"schmutzfink/internal/records"
)

func (s *Server) embeddingsStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"state": "off"}
	if s.Clip != nil {
		st := s.Clip.Status()
		out["state"] = st.State
		if st.Error != "" {
			out["error"] = st.Error
		}
	}
	if pending, ready, failed, err := s.Records.EmbeddingCounts(r.Context(), principal(r).TenantID); err == nil {
		out["pending"] = pending
		out["ready"] = ready
		out["failed"] = failed
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listWithOptionalSemantic(w http.ResponseWriter, r *http.Request, forMap bool) {
	f, semantic, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var (
		items []records.Record
		total int
	)
	if semantic && f.Query != "" {
		if err := s.requireEmbeddings(w); err != nil {
			return
		}
		vec, err := s.Clip.EmbedText(f.Query)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "Bildsuche ist gerade nicht verfügbar")
			return
		}
		if forMap {
			gps := true
			f.HasGPS = &gps
			f.Limit = 2000
			f.Offset = 0
		}
		items, total, err = s.Records.SearchByVector(r.Context(), principal(r).TenantID, vec, f, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Suche fehlgeschlagen")
			return
		}
	} else if forMap {
		items, err = s.Records.MapPoints(r.Context(), principal(r).TenantID, f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Kartenpunkte konnten nicht geladen werden")
			return
		}
		total = len(items)
	} else {
		items, total, err = s.Records.List(r.Context(), principal(r).TenantID, f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Datensätze konnten nicht geladen werden")
			return
		}
	}

	if forMap {
		writeJSON(w, http.StatusOK, map[string]any{"items": presentRecords(items)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": presentRecords(items), "total": total})
}

func (s *Server) searchByImage(w http.ResponseWriter, r *http.Request) {
	if err := s.requireEmbeddings(w); err != nil {
		return
	}
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
	img, err := ingest.Decode(data)
	if err != nil {
		if errors.Is(err, ingest.ErrTooManyPixels) {
			writeError(w, http.StatusBadRequest, "Bildauflösung zu hoch")
			return
		}
		if strings.Contains(err.Error(), "unsupported") {
			writeError(w, http.StatusBadRequest, "Nur JPEG, PNG, GIF oder WebP")
			return
		}
		writeError(w, http.StatusBadRequest, "Bild konnte nicht gelesen werden")
		return
	}
	vec, err := s.Clip.EmbedImage(img)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Bildsuche ist gerade nicht verfügbar")
		return
	}
	f, _, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	forMap := r.URL.Query().Get("for_map") == "true" || r.URL.Query().Get("for_map") == "1"
	if forMap {
		gps := true
		f.HasGPS = &gps
		f.Limit = 2000
		f.Offset = 0
	}
	items, total, err := s.Records.SearchByVector(r.Context(), principal(r).TenantID, vec, f, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Suche fehlgeschlagen")
		return
	}
	if forMap {
		writeJSON(w, http.StatusOK, map[string]any{"items": presentRecords(items)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": presentRecords(items), "total": total})
}

func (s *Server) requireEmbeddings(w http.ResponseWriter) error {
	if s.Clip == nil || !s.Clip.Ready() {
		msg := "Bildsuche wird noch vorbereitet. Bitte später erneut versuchen."
		if s.Clip != nil {
			if st := s.Clip.Status(); st.State == "error" {
				msg = "Bildsuche ist nicht verfügbar."
			}
		}
		writeError(w, http.StatusServiceUnavailable, msg)
		return clip.ErrNotReady
	}
	return nil
}
