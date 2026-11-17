package clip

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func applyBundledAssetEnv() {
	setEnvIfEmpty("ONNXRUNTIME_OPENCLIP_CACHE_DIR", findDir("vendor/openclip"))
	setEnvIfEmpty("ONNXRUNTIME_LIB_PATH", findORTLibrary())
	setEnvIfEmpty("TOKENIZERS_LIB_PATH", findTokenizerLibrary())
}

func bundledAssetsPresent() bool {
	cache := strings.TrimSpace(os.Getenv("ONNXRUNTIME_OPENCLIP_CACHE_DIR"))
	ortLib := strings.TrimSpace(os.Getenv("ONNXRUNTIME_LIB_PATH"))
	if cache == "" || ortLib == "" {
		return false
	}
	if _, err := os.Stat(ortLib); err != nil {
		return false
	}
	matches, err := filepath.Glob(filepath.Join(cache, "*", "*", "vision_model.onnx"))
	return err == nil && len(matches) > 0
}

func findORTLibrary() string {
	if runtime.GOOS == "windows" {
		return findFile("vendor/onnxruntime/onnxruntime.dll")
	}
	return findFile(
		"vendor/onnxruntime/libonnxruntime.so",
		"vendor/onnxruntime/libonnxruntime.so.1",
	)
}

func findTokenizerLibrary() string {
	if runtime.GOOS == "windows" {
		return findFile("vendor/tokenizers/tokenizers.dll")
	}
	return findFile("vendor/tokenizers/libtokenizers.so")
}

func setEnvIfEmpty(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) != "" || value == "" {
		return
	}
	_ = os.Setenv(key, value)
}

func findDir(rel string) string {
	for _, root := range searchRoots() {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

func findFile(rels ...string) string {
	for _, rel := range rels {
		for _, root := range searchRoots() {
			p := filepath.Join(root, filepath.FromSlash(rel))
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

func searchRoots() []string {
	roots := []string{"."}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	return roots
}
