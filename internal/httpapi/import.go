package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/ingest"
)

func (s *Server) importArchive(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, ingest.MaxZipBytes+1<<20)
	if err := r.ParseMultipartForm(ingest.MaxZipBytes); err != nil {
		writeError(w, http.StatusBadRequest, "ZIP zu groß oder ungültig")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "ZIP fehlt")
		return
	}
	defer file.Close()

	data, err := ingest.ReadAtMost(file, ingest.MaxZipBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ZIP zu groß")
		return
	}

	items, issues, err := ingest.ParseImportZip(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	p := principal(r)
	type itemErr struct {
		Filename string `json:"filename"`
		Error    string `json:"error"`
	}
	outErrors := make([]itemErr, 0, len(issues))
	for _, issue := range issues {
		outErrors = append(outErrors, itemErr{Filename: issue.Filename, Error: issue.Message})
	}

	imported := make([]map[string]any, 0, len(items))
	skipped := 0
	created := 0
	for _, item := range items {
		stored, err := s.savePhoto(r.Context(), p, item.Data, photoInput{
			Lat:        item.Lat,
			Lon:        item.Lon,
			CapturedAt: item.CapturedAt,
			Note:       item.Note,
			Tags:       item.Tags,
			Extra:      item.Extra,
		})
		if err != nil {
			var dup *duplicateError
			if errors.As(err, &dup) {
				skipped++
				continue
			}
			outErrors = append(outErrors, itemErr{Filename: item.Filename, Error: importItemError(err)})
			continue
		}
		created++
		imported = append(imported, presentPhoto(stored))
	}

	if created == 0 && skipped == 0 {
		writeError(w, http.StatusBadRequest, "Keine Bilder importiert")
		return
	}
	if created > 0 && s.Jobs != nil {
		s.Jobs.Notify()
	}
	s.writeAudit(r, p, audit.RecordImport, "", map[string]any{
		"imported": created,
		"skipped":  skipped,
		"errors":   len(outErrors),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": created,
		"skipped":  skipped,
		"errors":   outErrors,
		"items":    imported,
	})
}

func importItemError(err error) string {
	if err == nil {
		return "unbekannt"
	}
	if errors.Is(err, ingest.ErrTooManyPixels) {
		return "Bildauflösung zu hoch"
	}
	if strings.Contains(err.Error(), "unsupported type") {
		return "Kein unterstütztes Bild"
	}
	if strings.Contains(err.Error(), "file too large") {
		return "Datei zu groß"
	}
	return "Bild konnte nicht gespeichert werden"
}
