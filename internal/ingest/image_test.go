package ingest

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/disintegration/imaging"
)

func TestPixelCountOK(t *testing.T) {
	if !pixelCountOK(4000, 3000) {
		t.Fatal("12MP should be allowed")
	}
	if pixelCountOK(20000, 20000) {
		t.Fatal("400MP should be rejected")
	}
	if pixelCountOK(0, 100) {
		t.Fatal("zero width")
	}
}

func TestCropNorm(t *testing.T) {
	img := imaging.New(100, 80, color.NRGBA{R: 10, A: 255})
	out := CropNorm(img, 0.1, 0.25, 0.5, 0.5)
	b := out.Bounds()
	if b.Dx() != 50 || b.Dy() != 40 {
		t.Fatalf("got %dx%d", b.Dx(), b.Dy())
	}
}

func TestNormalizeForStoreKeepsSmall(t *testing.T) {
	img := imaging.New(800, 600, color.NRGBA{R: 20, G: 40, B: 60, A: 255})
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(90)); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	out, ct, err := NormalizeForStore(raw, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/jpeg" {
		t.Fatalf("ct %s", ct)
	}
	if !bytes.Equal(out, raw) {
		t.Fatal("small jpeg should be unchanged")
	}
}

func TestNormalizeForStoreShrinksLarge(t *testing.T) {
	img := imaging.New(4000, 3000, color.NRGBA{R: 20, G: 40, B: 60, A: 255})
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(90)); err != nil {
		t.Fatal(err)
	}
	out, ct, err := NormalizeForStore(buf.Bytes(), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/jpeg" {
		t.Fatalf("ct %s", ct)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width > MaxStoreEdge || cfg.Height > MaxStoreEdge {
		t.Fatalf("still too large: %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.Width != MaxStoreEdge && cfg.Height != MaxStoreEdge {
		t.Fatalf("expected one edge %d, got %dx%d", MaxStoreEdge, cfg.Width, cfg.Height)
	}
	ratio := float64(cfg.Width) / float64(cfg.Height)
	if ratio < 1.3 || ratio > 1.4 {
		t.Fatalf("aspect ratio drifted: %dx%d", cfg.Width, cfg.Height)
	}
}
