package ingest

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/webp"
)

const (
	MaxBytes  = 25 << 20
	MaxPixels = 40_000_000
)

type Meta struct {
	ContentType string
	Lat         *float64
	Lon         *float64
	CapturedAt  *time.Time
}

func Sniff(header []byte) (string, error) {
	ct := http.DetectContentType(header)
	switch ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return ct, nil
	default:
		if strings.HasPrefix(ct, "image/jpeg") {
			return "image/jpeg", nil
		}
		return "", fmt.Errorf("unsupported type %s", ct)
	}
}

func ReadLimited(r io.Reader) ([]byte, error) {
	return ReadAtMost(r, MaxBytes)
}

func ReadAtMost(r io.Reader, max int64) ([]byte, error) {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if n > max {
		return nil, fmt.Errorf("file too large")
	}
	return buf.Bytes(), nil
}

func ExtractJPEG(data []byte) Meta {
	meta := Meta{ContentType: "image/jpeg"}
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return meta
	}
	if lat, lon, err := x.LatLong(); err == nil {
		meta.Lat = &lat
		meta.Lon = &lon
	}
	if dt, err := x.DateTime(); err == nil {
		meta.CapturedAt = &dt
	}
	return meta
}

func Thumbnail(data []byte) ([]byte, error) {
	img, err := Decode(data)
	if err != nil {
		return nil, err
	}
	return EncodeThumbnail(img)
}

func ThumbnailCrop(data []byte, x, y, w, h float64) ([]byte, error) {
	img, err := Decode(data)
	if err != nil {
		return nil, err
	}
	return EncodeThumbnail(CropNorm(img, x, y, w, h))
}

func EncodeThumbnail(img image.Image) ([]byte, error) {
	thumb := imaging.Fit(img, 480, 480, imaging.Lanczos)
	var out bytes.Buffer
	if err := imaging.Encode(&out, thumb, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
