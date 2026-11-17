package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
)

func TestParseImportZip(t *testing.T) {
	jpeg := tinyJPEG(t)
	index, _ := json.Marshal([]map[string]any{
		{
			"id":       "wall_001",
			"filename": "wand.jpg",
			"location": map[string]any{
				"address": map[string]any{
					"street":      "Am Ellernbusch 6",
					"city":        "Hemmingen",
					"postal_code": "30966",
					"country":     "Germany",
				},
				"gps": map[string]any{"latitude": 52.3, "longitude": 9.7},
			},
			"metadata": map[string]any{
				"surface_type": "Brick",
				"created_at":   "2026-09-11T14:30:00Z",
				"notes":        "Ziegelwand",
			},
		},
	})
	raw, err := writeTestZip(map[string][]byte{
		"wand.jpg":     jpeg,
		"index.json":   index,
		"extra.jpg":    jpeg,
		"__MACOSX/._x": []byte("x"),
	})
	if err != nil {
		t.Fatal(err)
	}
	items, issues, err := ParseImportZip(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d", len(items))
	}
	got := items[0]
	if got.ImportID != "wall_001" || got.Filename != "wand.jpg" {
		t.Fatalf("identity %+v", got)
	}
	if got.Lat == nil || *got.Lat != 52.3 || got.Lon == nil || *got.Lon != 9.7 {
		t.Fatalf("gps %+v %+v", got.Lat, got.Lon)
	}
	if got.CapturedAt == nil || got.CapturedAt.UTC().Format("2006-01-02") != "2026-09-11" {
		t.Fatalf("captured %+v", got.CapturedAt)
	}
	if !strings.Contains(got.Note, "Am Ellernbusch 6, 30966 Hemmingen") || !strings.Contains(got.Note, "Ziegelwand") || !strings.Contains(got.Note, " — ") {
		t.Fatalf("note %q", got.Note)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "Hemmingen" || got.Tags[1] != "Brick" {
		t.Fatalf("tags %+v", got.Tags)
	}
	if extra, _ := got.Extra["import"].(map[string]any); extra["id"] != "wall_001" {
		t.Fatalf("extra %+v", got.Extra)
	}
	if len(issues) != 1 || issues[0].Filename != "extra.jpg" {
		t.Fatalf("issues %+v", issues)
	}
}

func TestParseImportZipRequiresIndex(t *testing.T) {
	raw, err := writeTestZip(map[string][]byte{"a.jpg": tinyJPEG(t)})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ParseImportZip(bytes.NewReader(raw), int64(len(raw)))
	if err == nil || !strings.Contains(err.Error(), "index.json") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseTagsExampleZip(t *testing.T) {
	path := filepath.Join("..", "..", "Beispiele", "import", "tags.zip")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip(err)
	}
	items, issues, err := ParseImportZip(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues %+v", issues)
	}
	if len(items) != 6 {
		t.Fatalf("items=%d", len(items))
	}
	byName := map[string]ImportItem{}
	for _, it := range items {
		byName[it.Filename] = it
	}
	ellern := byName["ellernbusch.jpg"]
	if ellern.ImportID != "wall_001" || ellern.Lat == nil || *ellern.Lat < 52 || ellern.Lon == nil {
		t.Fatalf("ellernbusch %+v", ellern)
	}
	if !strings.Contains(ellern.Note, "Hemmingen") || !strings.Contains(ellern.Note, "Ziegelwand") {
		t.Fatalf("note %q", ellern.Note)
	}
	if _, ok := byName["konradstraße.jpg"]; !ok {
		t.Fatalf("umlaut filename missing: %v", keys(byName))
	}
	if _, ok := byName["Mühlenholzweg.jpg"]; !ok {
		t.Fatalf("umlaut filename missing: %v", keys(byName))
	}
}

func TestSHA256HexStable(t *testing.T) {
	a := SHA256Hex([]byte("abc"))
	b := SHA256Hex([]byte("abc"))
	c := SHA256Hex([]byte("abd"))
	if a != b || a == c || len(a) != 64 {
		t.Fatalf("%s %s %s", a, b, c)
	}
}

func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := imaging.New(8, 8, color.NRGBA{R: 180, G: 20, B: 20, A: 255})
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(80)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func keys(m map[string]ImportItem) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestZipWriterRoundtripNames(t *testing.T) {
	raw, err := writeTestZip(map[string][]byte{"index.json": []byte("[]")})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if zr.File[0].Name != "index.json" {
		t.Fatal(zr.File[0].Name)
	}
}
