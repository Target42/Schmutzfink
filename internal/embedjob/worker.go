package embedjob

import (
	"context"
	"io"
	"log"
	"time"

	"schmutzfink/internal/clip"
	"schmutzfink/internal/ingest"
	"schmutzfink/internal/records"
	"schmutzfink/internal/storage"
)

type Worker struct {
	Repo  *records.Repo
	Store storage.Store
	Clip  *clip.Engine
	wake  chan struct{}
}

func New(repo *records.Repo, store storage.Store, engine *clip.Engine) *Worker {
	return &Worker{
		Repo:  repo,
		Store: store,
		Clip:  engine,
		wake:  make(chan struct{}, 1),
	}
}

func (w *Worker) Notify() {
	if w == nil {
		return
	}
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.Clip == nil {
		return
	}
	tick := time.NewTicker(4 * time.Second)
	defer tick.Stop()
	for {
		if w.Clip.Ready() {
			if err := w.Repo.RequeueStuck(ctx, 15*time.Minute); err != nil {
				log.Printf("embedding worker: %v", err)
			}
			w.drain(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-tick.C:
		}
	}
}

func (w *Worker) drain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		id, objectID, roi, gen, err := w.Repo.ClaimPending(ctx)
		if err != nil {
			log.Printf("embedding worker: %v", err)
			return
		}
		if id == "" {
			return
		}
		if err := w.embedOne(ctx, id, objectID, roi, gen); err != nil {
			log.Printf("embedding %s: %v", id, err)
			_ = w.Repo.FailEmbedding(ctx, id, gen, err.Error())
			continue
		}
		log.Printf("embedding %s: ready", id)
	}
}

func (w *Worker) embedOne(ctx context.Context, id, objectID string, roi *records.ROI, gen int) error {
	rc, err := w.Store.Open(ctx, objectID)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return err
	}
	img, err := ingest.Decode(data)
	if err != nil {
		return err
	}
	if roi != nil {
		img = ingest.CropNorm(img, roi.X, roi.Y, roi.W, roi.H)
	}
	vec, err := w.Clip.EmbedImage(img)
	if err != nil {
		return err
	}
	return w.Repo.SaveEmbedding(ctx, id, gen, vec)
}
