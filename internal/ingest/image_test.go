package ingest

import (
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
