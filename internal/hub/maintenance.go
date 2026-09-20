package hub

import (
	"context"
	"log"
	"time"
)

func (h *Hub) Maintenance(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		h.mu.Lock()
		e := prune(h.DB)
		h.mu.Unlock()
		if e != nil {
			log.Print("retention cleanup failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-h.quit:
			return
		case <-ticker.C:
		}
	}
}
