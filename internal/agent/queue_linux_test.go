//go:build linux

package agent

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestQueuePhysicalReservation(t *testing.T) {
	q := testQueue(t, queueCapacity)
	st, e := q.f.Stat()
	if e != nil {
		t.Fatal(e)
	}
	if st.Sys().(*syscall.Stat_t).Blocks*512 < queueCapacity {
		t.Fatal("queue is sparse rather than reserved")
	}
}

// Run only on a dedicated small tmpfs mounted by the test harness. Never fill
// the user's home, a shared /tmp, or /dev/shm to exercise this failure mode.
func TestQueueOnFullFilesystem(t *testing.T) {
	dir := os.Getenv("CARDSCOPE_FULL_DISK_DIR")
	if dir == "" {
		t.Skip("requires isolated test filesystem")
	}
	path := filepath.Join(dir, "queue.bin")
	q, e := openQueue(path, 4*1024*1024)
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	filler, e := os.Create(filepath.Join(dir, "filler"))
	if e != nil {
		t.Fatal(e)
	}
	defer filler.Close()
	block := make([]byte, 4096)
	for {
		_, e = filler.Write(block)
		if e != nil {
			break
		}
	}
	if !errors.Is(e, syscall.ENOSPC) {
		t.Fatal(e)
	}
	// Cycle beyond capacity while there is no space left to allocate.
	for i := int64(0); i < 1100; i++ {
		enqueueTestSample(t, q, i, time.Now())
	}
	if q.Dropped() == 0 {
		t.Fatal("did not exercise wraparound")
	}
	for _, r := range q.batch() {
		if e = q.ack(r); e != nil {
			t.Fatal(e)
		}
	}
	q.Close()
	again, e := openQueue(path, 4*1024*1024)
	if e != nil {
		t.Fatal(e)
	}
	defer again.Close()
	if len(again.records) == 0 {
		t.Fatal("reopen lost queue")
	}
	if _, e = openQueue(filepath.Join(dir, "cannot-allocate.bin"), 4*1024*1024); e == nil {
		t.Fatal("reservation succeeded on full filesystem")
	}
	if _, e = os.Stat(filepath.Join(dir, "cannot-allocate.bin")); !os.IsNotExist(e) {
		t.Fatal("failed allocation left partial file")
	}
}
