package webui

import (
	"bytes"
	"embed"
	"io/fs"
)

//go:embed all:dist
var Dist embed.FS

func Files() (fs.FS, error) {
	return fs.Sub(Dist, "dist")
}

func Built() bool {
	b, err := Dist.ReadFile("dist/index.html")
	if err != nil {
		return false
	}
	return bytes.Contains(b, []byte("/assets/"))
}
