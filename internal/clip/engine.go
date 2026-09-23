package clip

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amikos-tech/pure-onnx/embeddings/openclip"
	"github.com/amikos-tech/pure-onnx/ort"
	"github.com/disintegration/imaging"
)

const Dim = openclip.OutputEmbeddingDimension

var ErrNotReady = errors.New("bildsuche noch nicht bereit")

type Status struct {
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

type Engine struct {
	mu     sync.RWMutex
	state  string
	err    error
	inner  *openclip.Embedder
	cancel context.CancelFunc
}

func New(enabled bool) *Engine {
	e := &Engine{state: "off"}
	if !enabled {
		return e
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.state = "loading"
	go e.load(ctx)
	return e
}

func (e *Engine) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	st := Status{State: e.state}
	if e.err != nil {
		st.Error = e.err.Error()
	}
	return st
}

func (e *Engine) Ready() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.inner != nil && e.state == "ready"
}

func (e *Engine) EmbedImage(img image.Image) ([]float32, error) {
	inner, err := e.embedder()
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, fmt.Errorf("bild fehlt")
	}
	if img.Bounds().Dx() > 1024 || img.Bounds().Dy() > 1024 {
		img = imaging.Fit(img, 1024, 1024, imaging.Box)
	}
	return inner.EmbedImage(img)
}

func (e *Engine) EmbedText(text string) ([]float32, error) {
	inner, err := e.embedder()
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("suchtext fehlt")
	}
	return inner.EmbedText(text)
}

func (e *Engine) Close() {
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inner != nil {
		_ = e.inner.Close()
		e.inner = nil
	}
	if ort.IsInitialized() {
		_ = ort.DestroyEnvironment()
	}
}

func (e *Engine) embedder() (*openclip.Embedder, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.inner == nil {
		if e.err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNotReady, e.err)
		}
		return nil, ErrNotReady
	}
	return e.inner, nil
}

func (e *Engine) load(ctx context.Context) {
	applyBundledAssetEnv()
	if bundledAssetsPresent() {
		log.Print("CLIP/ONNX: mitgelieferte Runtime und Modelldateien werden geladen")
	} else {
		log.Print("CLIP/ONNX: Runtime und Modell werden geladen (erster Start kann mehrere Minuten dauern)")
	}
	if err := initORT(); err != nil {
		e.fail(err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	assets, err := downloadAssets(ctx)
	if err != nil {
		e.fail(fmt.Errorf("modelldateien: %w", err))
		return
	}
	if ctx.Err() != nil {
		return
	}
	opts := []openclip.Option{}
	if lib := strings.TrimSpace(os.Getenv("TOKENIZERS_LIB_PATH")); lib != "" {
		opts = append(opts, openclip.WithTokenizerLibraryPath(lib))
	}
	inner, err := openclip.NewEmbedder(
		assets.TextModelPath,
		assets.VisionModelPath,
		assets.TokenizerPath,
		assets.PreprocessorConfigPath,
		opts...,
	)
	if err != nil {
		e.fail(fmt.Errorf("embedder: %w", err))
		return
	}
	e.mu.Lock()
	e.inner = inner
	e.state = "ready"
	e.err = nil
	e.mu.Unlock()
	log.Print("CLIP/ONNX: bereit")
}

func downloadAssets(ctx context.Context) (openclip.ModelAssets, error) {
	wait := 15 * time.Second
	var last error
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			if last != nil {
				return openclip.ModelAssets{}, last
			}
			return openclip.ModelAssets{}, ctx.Err()
		}
		assets, err := openclip.EnsureDefaultAssets()
		if err == nil {
			return assets, nil
		}
		last = err
		log.Printf("CLIP/ONNX: Modelldownload fehlgeschlagen (%d): %v — neuer Versuch in %s", attempt, err, wait)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return openclip.ModelAssets{}, last
		case <-timer.C:
		}
		if wait < 5*time.Minute {
			wait *= 2
			if wait > 5*time.Minute {
				wait = 5 * time.Minute
			}
		}
	}
}

func (e *Engine) fail(err error) {
	log.Printf("CLIP/ONNX: nicht verfügbar: %v", err)
	e.mu.Lock()
	e.state = "error"
	e.err = err
	e.mu.Unlock()
}

func initORT() error {
	libPath := strings.TrimSpace(os.Getenv("ONNXRUNTIME_LIB_PATH"))
	if libPath != "" {
		if err := ort.SetSharedLibraryPath(libPath); err != nil {
			return err
		}
		return ort.InitializeEnvironment()
	}
	bootstrapped, err := ort.EnsureOnnxRuntimeSharedLibrary()
	if err != nil {
		return fmt.Errorf("onnxruntime.dll: %w", err)
	}
	log.Printf("CLIP/ONNX: Runtime %s", bootstrapped)
	if err := ort.SetSharedLibraryPath(bootstrapped); err != nil {
		return err
	}
	return ort.InitializeEnvironment()
}
