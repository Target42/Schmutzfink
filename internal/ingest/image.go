package ingest

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/disintegration/imaging"
)

var ErrTooManyPixels = errors.New("image too large")

func CheckDimensions(data []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if !pixelCountOK(cfg.Width, cfg.Height) {
		return fmt.Errorf("%w: %dx%d", ErrTooManyPixels, cfg.Width, cfg.Height)
	}
	return nil
}

func Decode(data []byte) (image.Image, error) {
	if err := CheckDimensions(data); err != nil {
		return nil, err
	}
	return imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
}

// NormalizeForStore shrinks oversized images for storage. EXIF/GPS should be read from
// the raw upload first; the returned JPEG has orientation baked in and no EXIF.
// Images already within MaxStoreEdge are returned unchanged.
func NormalizeForStore(data []byte, contentType string) ([]byte, string, error) {
	img, err := Decode(data)
	if err != nil {
		return nil, "", err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= MaxStoreEdge && h <= MaxStoreEdge {
		return data, contentType, nil
	}
	img = imaging.Fit(img, MaxStoreEdge, MaxStoreEdge, imaging.Lanczos)
	var out bytes.Buffer
	if err := imaging.Encode(&out, img, imaging.JPEG, imaging.JPEGQuality(StoreJPEGQuality)); err != nil {
		return nil, "", err
	}
	return out.Bytes(), "image/jpeg", nil
}

func pixelCountOK(w, h int) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	return int64(w)*int64(h) <= MaxPixels
}

func CropNorm(img image.Image, x, y, nw, nh float64) image.Image {
	if img == nil {
		return img
	}
	b := img.Bounds()
	fw := float64(b.Dx())
	fh := float64(b.Dy())
	if fw < 2 || fh < 2 {
		return img
	}
	x0 := b.Min.X + int(math.Round(clamp01(x)*fw))
	y0 := b.Min.Y + int(math.Round(clamp01(y)*fh))
	x1 := b.Min.X + int(math.Round(clamp01(x+nw)*fw))
	y1 := b.Min.Y + int(math.Round(clamp01(y+nh)*fh))
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if x1-x0 < 2 || y1-y0 < 2 {
		return img
	}
	return imaging.Crop(img, image.Rect(x0, y0, x1, y1))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
