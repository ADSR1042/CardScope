package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"gpu-monitor/internal/model"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// uploadLoop sends the newest sample first, then replays queued samples.
func uploadLoop(ctx context.Context, c Config, dir string) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := uploadBatch(ctx, c, dir); err != nil {
				fmt.Fprintln(os.Stderr, time.Now().Format(time.RFC3339), err)
			}
		}
	}
}

// uploadBatch stops at the first failed upload, retaining that sample for retry.
func uploadBatch(ctx context.Context, c Config, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	// 每轮先发送最新样本恢复实时状态，再按时间顺序补传历史，最多尝试 20 个文件。
	if len(entries) > 1 {
		entries = append([]os.DirEntry{entries[len(entries)-1]}, entries[:len(entries)-1]...)
	}
	for i, f := range entries {
		if i >= 20 || ctx.Err() != nil {
			break
		}
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, f.Name())
		b, e := os.ReadFile(path)
		if e != nil {
			continue
		}
		var s model.Snapshot
		if json.Unmarshal(b, &s) != nil {
			continue
		}
		s.Backfill = time.Now().UnixMilli()-s.At > int64(c.Interval*2000)
		if e = post(c, "/snapshots", s, nil); e != nil {
			return e
		}
		os.Remove(path)
	}
	return nil
}
