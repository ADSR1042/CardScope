package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"gpu-monitor/internal/model"
	"os"
	"time"
)

// uploadLoop sends the newest sample first, then replays queued samples.
func uploadLoop(ctx context.Context, c Config, q *diskQueue) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := uploadBatch(ctx, c, q); err != nil {
				fmt.Fprintln(os.Stderr, time.Now().Format(time.RFC3339), err)
			}
		}
	}
}

// uploadBatch stops at the first failed upload, retaining that sample for retry.
func uploadBatch(ctx context.Context, c Config, q *diskQueue) error {
	for _, rec := range q.batch() {
		if ctx.Err() != nil {
			break
		}
		b, err := q.read(rec)
		if err != nil {
			return err
		}
		if b == nil {
			continue
		}
		var s model.Snapshot
		if err = json.Unmarshal(b, &s); err != nil {
			return err
		}
		s.Backfill = time.Now().UnixMilli()-s.At > int64(c.Interval*2000)
		if err = post(c, "/snapshots", s, nil); err != nil {
			return err
		}
		if err = q.ack(rec); err != nil {
			return err
		}
	}
	return nil
}
