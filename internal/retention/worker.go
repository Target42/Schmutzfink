package retention

import (
	"context"
	"log"
	"time"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/records"
)

func Run(ctx context.Context, repo *records.Repo, trail *audit.Repo) {
	if repo == nil {
		return
	}
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	apply := func() {
		items, err := repo.RedactDueLocations(ctx)
		if err != nil {
			log.Printf("löschfrist: %v", err)
			return
		}
		if len(items) == 0 {
			return
		}
		log.Printf("löschfrist: %d Standorte entfernt", len(items))
		for _, item := range items {
			if trail == nil {
				continue
			}
			_ = trail.Write(ctx, audit.Event{
				TenantID: item.TenantID,
				Username: "system",
				Action:   audit.LocationRedact,
				Subject:  item.PhotoID,
			})
		}
	}
	apply()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			apply()
		}
	}
}
