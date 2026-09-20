package agent

import (
	"context"
	"encoding/json"
	"gpu-monitor/internal/model"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

func enqueueTestSample(t *testing.T, q *diskQueue, seq int64, at time.Time) {
	t.Helper()
	b, err := json.Marshal(model.Snapshot{Seq: seq, At: at.UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	if err = q.push(b, at.UnixMilli()); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRetriesWithoutLosingSamples(t *testing.T) {
	q := testQueue(t, 64*queueBlock)
	enqueueTestSample(t, q, 1, time.Now().Add(-time.Minute))
	enqueueTestSample(t, q, 2, time.Now())
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
	if err := uploadBatch(context.Background(), c, q); err == nil {
		t.Fatal("failed upload returned success")
	}
	files := q.records
	if len(files) != 2 {
		t.Fatalf("failed upload lost queued records: %v", files)
	}
	if err := uploadBatch(context.Background(), c, q); err != nil {
		t.Fatal(err)
	}
	files = q.records
	if len(files) != 0 {
		t.Fatalf("acknowledged records remain: %v", files)
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
	q := testQueue(t, 64*queueBlock)
	for i := int64(1); i <= 25; i++ {
		enqueueTestSample(t, q, i, time.Now())
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	c := Config{Server: server.URL, Interval: 5}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := uploadBatch(ctx, c, q); err != nil {
		t.Fatal(err)
	}
	files := q.records
	if len(files) != 25 {
		t.Fatal("cancelled batch consumed samples")
	}
	if err := uploadBatch(context.Background(), c, q); err != nil {
		t.Fatal(err)
	}
	files = q.records
	if len(files) != 5 {
		t.Fatalf("batch should send 20 samples, left %d", len(files))
	}
}

func TestUploadRedirectRetainsRecord(t *testing.T) {
	q := testQueue(t, 4*queueBlock)
	enqueueTestSample(t, q, 1, time.Now())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	if e := uploadBatch(context.Background(), Config{Server: server.URL, Interval: 5}, q); e == nil {
		t.Fatal("redirect acknowledged sample")
	}
	if len(q.records) != 1 {
		t.Fatal("redirect lost sample")
	}
}

func TestUploadAckAfterConcurrentOverwrite(t *testing.T) {
	q := testQueue(t, 3*queueBlock)
	enqueueTestSample(t, q, 1, time.Now())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enqueueTestSample(t, q, 2, time.Now())
		enqueueTestSample(t, q, 3, time.Now())
	}))
	defer server.Close()
	if e := uploadBatch(context.Background(), Config{Server: server.URL, Interval: 5}, q); e != nil {
		t.Fatal(e)
	}
	if len(q.records) != 2 {
		t.Fatal("in-flight old response deleted new sample")
	}
}
