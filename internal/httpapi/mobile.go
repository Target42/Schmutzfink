package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type androidMobileInfo struct {
	Available   bool   `json:"available"`
	Version     string `json:"version,omitempty"`
	VersionCode int    `json:"version_code,omitempty"`
	ReleasedAt  string `json:"released_at,omitempty"`
	Notes       string `json:"notes,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	Filename    string `json:"filename,omitempty"`
}

type androidMobileMeta struct {
	Version     string `json:"version"`
	VersionCode int    `json:"version_code"`
	ReleasedAt  string `json:"released_at"`
	Notes       string `json:"notes"`
}

const androidAPKFilename = "schmutzfink-android.apk"

func (s *Server) mobileAndroidInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.loadAndroidMobileInfo())
}

func (s *Server) downloadAndroidAPK(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(s.MobileAPKPath)
	if path == "" {
		writeError(w, http.StatusNotFound, "Android-App ist nicht hinterlegt")
		return
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		writeError(w, http.StatusNotFound, "Android-App ist nicht hinterlegt")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "Android-App ist nicht hinterlegt")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", `attachment; filename="`+androidAPKFilename+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, androidAPKFilename, st.ModTime(), f)
}

func (s *Server) loadAndroidMobileInfo() androidMobileInfo {
	path := strings.TrimSpace(s.MobileAPKPath)
	if path == "" {
		return androidMobileInfo{Available: false}
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return androidMobileInfo{Available: false}
	}
	info := androidMobileInfo{
		Available:  true,
		SizeBytes:  st.Size(),
		Filename:   androidAPKFilename,
		ReleasedAt: st.ModTime().UTC().Format(time.RFC3339),
	}
	if meta, ok := readAndroidMobileMeta(androidMetaPath(path)); ok {
		if meta.Version != "" {
			info.Version = meta.Version
		}
		if meta.VersionCode > 0 {
			info.VersionCode = meta.VersionCode
		}
		if meta.ReleasedAt != "" {
			info.ReleasedAt = meta.ReleasedAt
		}
		if meta.Notes != "" {
			info.Notes = meta.Notes
		}
	}
	return info
}

func androidMetaPath(apkPath string) string {
	ext := filepath.Ext(apkPath)
	return strings.TrimSuffix(apkPath, ext) + ".json"
}

func readAndroidMobileMeta(path string) (androidMobileMeta, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return androidMobileMeta{}, false
	}
	var meta androidMobileMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return androidMobileMeta{}, false
	}
	return meta, true
}
