package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"gpu-monitor/internal/model"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func enqueueTestSample(t *testing.T, dir string, seq int64, at time.Time) {
	t.Helper()
	b, err := json.Marshal(model.Snapshot{Seq: seq, At: at.UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%03d.json", seq)), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRetriesWithoutLosingSamples(t *testing.T) {
	dir := t.TempDir()
	enqueueTestSample(t, dir, 1, time.Now().Add(-time.Minute))
	enqueueTestSample(t, dir, 2, time.Now())
	var mu sync.Mutex
	var received []int64
	var backfill []bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/snapshots" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("wrong upload endpoint or credentials")
		}
		var sample model.Snapshot
		if err := json.NewDecoder(r.Body).Decode(&sample); err != nil {
			t.Error(err)
		}
		mu.Lock()
		received = append(received, sample.Seq)
		backfill = append(backfill, sample.Backfill)
		first := len(received) == 1
		mu.Unlock()
		if first {
			http.Error(w, "retry later", http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	c := Config{Server: server.URL, Token: "test-token", Interval: 5}
	if err := uploadBatch(context.Background(), c, dir); err == nil {
		t.Fatal("failed upload returned success")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("failed upload lost queued files: %v, %v", files, err)
	}
	if err := uploadBatch(context.Background(), c, dir); err != nil {
		t.Fatal(err)
	}
	files, err = os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("acknowledged files remain: %v, %v", files, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(received, []int64{2, 2, 1}) {
		t.Fatalf("upload order = %v", received)
	}
	if !reflect.DeepEqual(backfill, []bool{false, false, true}) {
		t.Fatalf("backfill flags = %v", backfill)
	}
}

func TestUploadBatchLimitAndCancellation(t *testing.T) {
	dir := t.TempDir()
	for i := int64(1); i <= 25; i++ {
		enqueueTestSample(t, dir, i, time.Now())
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	c := Config{Server: server.URL, Interval: 5}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := uploadBatch(ctx, c, dir); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 25 {
		t.Fatal("cancelled batch consumed samples")
	}
	if err := uploadBatch(context.Background(), c, dir); err != nil {
		t.Fatal(err)
	}
	files, _ = os.ReadDir(dir)
	if len(files) != 5 {
		t.Fatalf("batch should send 20 samples, left %d", len(files))
	}
}
