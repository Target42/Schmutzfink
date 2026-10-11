package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

const (
	MaxZipBytes = 100 << 20
	MaxZipFiles = 500
)

type ImportItem struct {
	Filename   string
	ImportID   string
	Data       []byte
	Lat        *float64
	Lon        *float64
	CapturedAt *time.Time
	Note       string
	Tags       []string
	Extra      map[string]any
}

type ImportIssue struct {
	Filename string
	Message  string
}

type zipCatalogEntry struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Location struct {
		Address zipAddress `json:"address"`
		GPS     struct {
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
		} `json:"gps"`
	} `json:"location"`
	Metadata *zipMetadata `json:"metadata"`
}

type zipAddress struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type zipMetadata struct {
	SurfaceType string `json:"surface_type"`
	CreatedAt   string `json:"created_at"`
	Notes       string `json:"notes"`
}

func ParseImportZip(r io.ReaderAt, size int64) ([]ImportItem, []ImportIssue, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, nil, fmt.Errorf("keine ZIP-Datei")
	}
	if len(zr.File) > MaxZipFiles {
		return nil, nil, fmt.Errorf("ZIP enthält zu viele Dateien")
	}

	var catalogRaw []byte
	files := make(map[string][]byte, len(zr.File))
	var uncompressed uint64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := zipBase(f.Name)
		if name == "" || strings.HasPrefix(name, ".") || strings.EqualFold(name, "thumbs.db") {
			continue
		}
		if strings.Contains(strings.ToLower(f.Name), "__macosx/") {
			continue
		}
		if f.UncompressedSize64 > MaxBytes {
			return nil, nil, fmt.Errorf("%s ist größer als erlaubt", name)
		}
		uncompressed += f.UncompressedSize64
		if uncompressed > uint64(MaxZipBytes)*2 {
			return nil, nil, fmt.Errorf("ZIP ist zu groß")
		}
		data, err := readZipFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("%s konnte nicht gelesen werden", name)
		}
		if strings.EqualFold(name, "index.json") {
			catalogRaw = data
			continue
		}
		key := strings.ToLower(name)
		if _, exists := files[key]; exists {
			return nil, nil, fmt.Errorf("doppelter Dateiname %s", name)
		}
		files[key] = data
	}
	if len(catalogRaw) == 0 {
		return nil, nil, fmt.Errorf("index.json fehlt")
	}

	var catalog []zipCatalogEntry
	if err := json.Unmarshal(catalogRaw, &catalog); err != nil {
		return nil, nil, fmt.Errorf("index.json ist ungültig")
	}
	if len(catalog) == 0 {
		return nil, nil, fmt.Errorf("index.json enthält keine Einträge")
	}

	var items []ImportItem
	var issues []ImportIssue
	seen := make(map[string]bool, len(catalog))
	for _, entry := range catalog {
		filename := strings.TrimSpace(entry.Filename)
		if filename == "" {
			issues = append(issues, ImportIssue{Message: "Eintrag ohne Dateiname"})
			continue
		}
		base := zipBase(filename)
		key := strings.ToLower(base)
		if seen[key] {
			issues = append(issues, ImportIssue{Filename: base, Message: "doppelt in index.json"})
			continue
		}
		seen[key] = true
		data, ok := files[key]
		if !ok {
			issues = append(issues, ImportIssue{Filename: base, Message: "Datei fehlt im ZIP"})
			continue
		}
		delete(files, key)
		items = append(items, catalogItem(entry, base, data))
	}
	for name := range files {
		issues = append(issues, ImportIssue{Filename: name, Message: "kein Eintrag in index.json"})
	}
	return items, issues, nil
}

func catalogItem(entry zipCatalogEntry, filename string, data []byte) ImportItem {
	addr := zipAddress{
		Street:     strings.TrimSpace(entry.Location.Address.Street),
		City:       strings.TrimSpace(entry.Location.Address.City),
		PostalCode: strings.TrimSpace(entry.Location.Address.PostalCode),
		Country:    strings.TrimSpace(entry.Location.Address.Country),
	}
	item := ImportItem{
		Filename: filename,
		ImportID: strings.TrimSpace(entry.ID),
		Data:     data,
		Note:     importNote(addr, entry.Metadata),
		Tags:     importTags(addr, entry.Metadata),
		Extra:    importExtra(entry, addr),
	}
	if lat, lon := entry.Location.GPS.Latitude, entry.Location.GPS.Longitude; lat != nil && lon != nil {
		item.Lat = lat
		item.Lon = lon
	}
	if entry.Metadata != nil {
		if raw := strings.TrimSpace(entry.Metadata.CreatedAt); raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				item.CapturedAt = &t
			}
		}
	}
	return item
}

func importNote(addr zipAddress, meta *zipMetadata) string {
	var lines []string
	if loc := formatAddress(addr); loc != "" {
		lines = append(lines, loc)
	}
	if meta != nil {
		if n := strings.TrimSpace(meta.Notes); n != "" {
			lines = append(lines, n)
		}
	}
	return strings.Join(lines, " — ")
}

func importTags(addr zipAddress, meta *zipMetadata) []string {
	var tags []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		for _, t := range tags {
			if strings.EqualFold(t, v) {
				return
			}
		}
		tags = append(tags, v)
	}
	add(addr.City)
	if meta != nil {
		add(meta.SurfaceType)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags
}

func importExtra(entry zipCatalogEntry, addr zipAddress) map[string]any {
	payload := map[string]any{
		"id":       strings.TrimSpace(entry.ID),
		"filename": strings.TrimSpace(entry.Filename),
	}
	address := map[string]any{}
	if addr.Street != "" {
		address["street"] = addr.Street
	}
	if addr.City != "" {
		address["city"] = addr.City
	}
	if addr.PostalCode != "" {
		address["postal_code"] = addr.PostalCode
	}
	if addr.Country != "" {
		address["country"] = addr.Country
	}
	if len(address) > 0 {
		payload["address"] = address
	}
	if entry.Metadata != nil {
		if v := strings.TrimSpace(entry.Metadata.SurfaceType); v != "" {
			payload["surface_type"] = v
		}
	}
	return map[string]any{"import": payload}
}

func formatAddress(addr zipAddress) string {
	var loc string
	switch {
	case addr.PostalCode != "" && addr.City != "":
		loc = addr.PostalCode + " " + addr.City
	case addr.City != "":
		loc = addr.City
	default:
		loc = addr.PostalCode
	}
	switch {
	case addr.Street != "" && loc != "":
		return addr.Street + ", " + loc
	case addr.Street != "":
		return addr.Street
	default:
		return loc
	}
}

func zipBase(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	return path.Base(name)
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	limit := int64(f.UncompressedSize64)
	if limit <= 0 || limit > MaxBytes {
		limit = MaxBytes
	}
	return ReadLimited(io.LimitReader(rc, limit))
}

func writeTestZip(items map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range items {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
