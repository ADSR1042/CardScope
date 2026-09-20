package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testQueue(t *testing.T, size int64) *diskQueue {
	t.Helper()
	q, e := openQueue(filepath.Join(t.TempDir(), "queue.bin"), size)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { q.Close() })
	return q
}
func TestQueueWrapAndStaleAck(t *testing.T) {
	q := testQueue(t, 4*queueBlock)
	for i := int64(1); i <= 3; i++ {
		enqueueTestSample(t, q, i, time.Now())
	}
	old := q.records[0]
	enqueueTestSample(t, q, 4, time.Now())
	if q.Dropped() != 1 || len(q.records) != 3 {
		t.Fatalf("dropped=%d records=%d", q.Dropped(), len(q.records))
	}
	if e := q.ack(old); e != nil {
		t.Fatal(e)
	}
	if len(q.records) != 3 {
		t.Fatal("stale ack removed replacement")
	}
	p := q.f.Name()
	q.Close()
	again, e := openQueue(p, 4*queueBlock)
	if e != nil {
		t.Fatal(e)
	}
	defer again.Close()
	if len(again.records) != 3 || again.Dropped() != 1 {
		t.Fatal("recovery lost records or counter")
	}
	st, _ := again.f.Stat()
	if st.Size() != 4*queueBlock {
		t.Fatal("file grew")
	}
}
func TestQueueVariableRecordsAndExpiry(t *testing.T) {
	q := testQueue(t, 8*queueBlock)
	enqueueTestSample(t, q, 1, time.Now().Add(-25*time.Hour))
	b, _ := json.Marshal(map[string]string{"large": string(bytes.Repeat([]byte("a"), 9000))})
	if e := q.push(b, time.Now().UnixMilli()); e != nil {
		t.Fatal(e)
	}
	if q.Dropped() != 1 {
		t.Fatal("old record not expired")
	}
	got, e := q.read(q.records[0])
	if e != nil || !bytes.Equal(b, got) {
		t.Fatal("multi-block payload changed")
	}
	q.Close()
	again, e := openQueue(q.f.Name(), 8*queueBlock)
	if e != nil {
		t.Fatal(e)
	}
	defer again.Close()
	if len(again.records) != 1 {
		t.Fatal("variable record not recovered")
	}
}
func TestQueueTornRecordAndCounter(t *testing.T) {
	q := testQueue(t, 8*queueBlock)
	enqueueTestSample(t, q, 1, time.Now())
	enqueueTestSample(t, q, 2, time.Now())
	r := q.records[1]
	if _, e := q.f.WriteAt([]byte("broken"), r.offset+recordHeader); e != nil {
		t.Fatal(e)
	}
	// Corrupt the most recent metadata copy: the previous copy stays usable.
	if _, e := q.f.WriteAt([]byte("broken"), 64+int64(q.serial%2)*64); e != nil {
		t.Fatal(e)
	}
	q.Close()
	again, e := openQueue(q.f.Name(), 8*queueBlock)
	if e != nil {
		t.Fatal(e)
	}
	defer again.Close()
	if len(again.records) != 1 {
		t.Fatal("torn payload accepted or intact sample lost")
	}
}
func TestQueueMigrationRestart(t *testing.T) {
	q := testQueue(t, 8*queueBlock)
	dir := filepath.Join(filepath.Dir(q.f.Name()), "queue")
	os.Mkdir(dir, 0700)
	b := []byte(`{"at":` + timeString() + `,"seq":1}`)
	if e := q.push(b, time.Now().UnixMilli()); e != nil {
		t.Fatal(e)
	} // Previous migration committed, but unlink did not happen.
	path := filepath.Join(dir, "001.json")
	os.WriteFile(path, b, 0600)
	if e := q.migrate(dir); e != nil {
		t.Fatal(e)
	}
	if len(q.records) != 1 {
		t.Fatal("migration duplicated sample")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("old copy remains")
	}
}
func timeString() string { b, _ := json.Marshal(time.Now().UnixMilli()); return string(b) }
func TestQueueRejectsBadFileAndOversizedSample(t *testing.T) {
	q := testQueue(t, 3*queueBlock)
	b, _ := json.Marshal(string(bytes.Repeat([]byte("x"), 3*queueBlock)))
	if q.push(b, time.Now().UnixMilli()) == nil {
		t.Fatal("oversized sample accepted")
	}
	if len(q.records) != 0 {
		t.Fatal("invalid push mutated records")
	}
	path := q.f.Name()
	q.Close()
	os.WriteFile(path, []byte("bad"), 0600)
	if _, e := openQueue(path, 3*queueBlock); e == nil {
		t.Fatal("corrupt file accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "bad" {
		t.Fatal("existing corrupt file destroyed")
	}
}

func TestQueueAcknowledgementSurvivesReopen(t *testing.T) {
	q := testQueue(t, 4*queueBlock)
	enqueueTestSample(t, q, 1, time.Now())
	enqueueTestSample(t, q, 2, time.Now())
	for _, r := range q.batch() {
		if e := q.ack(r); e != nil {
			t.Fatal(e)
		}
	}
	q.Close()
	again, e := openQueue(q.f.Name(), 4*queueBlock)
	if e != nil {
		t.Fatal(e)
	}
	defer again.Close()
	if len(again.records) != 0 {
		t.Fatal("acknowledged samples resurrected")
	}
}

func TestQueueMigrationKeepsInvalidSource(t *testing.T) {
	q := testQueue(t, 4*queueBlock)
	dir := filepath.Join(filepath.Dir(q.f.Name()), "queue")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "001.json")
	if e := os.WriteFile(path, []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := q.migrate(dir); e == nil {
		t.Fatal("invalid migration succeeded")
	}
	if b, e := os.ReadFile(path); e != nil || string(b) != "broken" {
		t.Fatal("failed migration removed source")
	}
}
