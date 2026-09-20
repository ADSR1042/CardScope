package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const queueCapacity = 100 * 1024 * 1024
const queueBlock = 4096
const recordHeader = 64

var recordMagic = []byte("CSREC001")
var queueMagic = []byte("CSQUEUE1")

type queueRecord struct {
	offset int64
	id     uint64
	size   int
	at     int64
}
type diskQueue struct {
	mu             sync.Mutex
	f              *os.File
	capacity, next int64
	serial         uint64
	dropped        int64
	records        []queueRecord
}

// Callers hold the agent lock for the lifetime of the queue. All mutations are
// in-place: no rename, growing WAL, or new sample file is needed after opening.
func openQueue(path string, capacity int64) (*diskQueue, error) {
	if capacity < 3*queueBlock || capacity%queueBlock != 0 {
		return nil, fmt.Errorf("invalid queue capacity")
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	fresh := err == nil
	if os.IsExist(err) {
		f, err = os.OpenFile(path, os.O_RDWR, 0600)
	}
	if err != nil {
		return nil, err
	}
	fail := func(e error) (*diskQueue, error) {
		f.Close()
		if fresh {
			os.Remove(path)
		}
		return nil, e
	}
	if fresh {
		if err = reserveQueue(f, capacity); err != nil {
			return fail(fmt.Errorf("无法预分配 %d MiB 客户端缓存: %w", capacity/1024/1024, err))
		}
		header := make([]byte, queueBlock)
		copy(header, queueMagic)
		binary.LittleEndian.PutUint64(header[8:], uint64(capacity))
		// Alternating checksummed counters allow recovery if one copy is torn.
		for _, pos := range []int{64, 128} {
			binary.LittleEndian.PutUint32(header[pos+16:], crc32.ChecksumIEEE(header[pos:pos+16]))
		}
		if _, err = f.WriteAt(header, 0); err != nil {
			return fail(err)
		}
		if err = f.Sync(); err != nil {
			return fail(err)
		}
		if err = syncQueueDirectory(filepath.Dir(path)); err != nil {
			return fail(err)
		}
	}
	st, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	if st.Size() != capacity {
		return fail(fmt.Errorf("缓存大小异常，保留原文件: %s", path))
	}
	header := make([]byte, queueBlock)
	if _, err = f.ReadAt(header, 0); err != nil {
		return fail(err)
	}
	if !bytes.Equal(header[:8], queueMagic) || binary.LittleEndian.Uint64(header[8:]) != uint64(capacity) {
		return fail(fmt.Errorf("缓存格式异常，保留原文件: %s", path))
	}
	// Backups or external copies may recreate a sparse file of the right size.
	if !fresh {
		if err = ensureQueueReservation(f, capacity); err != nil {
			return fail(fmt.Errorf("缓存空间预留失败: %w", err))
		}
	}
	q := &diskQueue{f: f, capacity: capacity, next: queueBlock}
	valid := false
	for _, pos := range []int{64, 128} {
		b := header[pos : pos+20]
		if crc32.ChecksumIEEE(b[:16]) == binary.LittleEndian.Uint32(b[16:]) {
			n := binary.LittleEndian.Uint64(b)
			if !valid || n >= q.serial {
				q.serial = n
				q.dropped = int64(binary.LittleEndian.Uint64(b[8:]))
				valid = true
			}
		}
	}
	if !valid {
		return fail(fmt.Errorf("缓存计数损坏，保留原文件"))
	}
	var newest uint64
	for off := int64(queueBlock); off < capacity; off += queueBlock {
		h := make([]byte, recordHeader)
		if _, err = f.ReadAt(h, off); err != nil {
			return fail(err)
		}
		if !bytes.Equal(h[:8], recordMagic) || crc32.ChecksumIEEE(h[:36]) != binary.LittleEndian.Uint32(h[36:]) {
			continue
		}
		size := int(binary.LittleEndian.Uint32(h[16:]))
		if size <= 0 || int64(size) > capacity-queueBlock-recordHeader || off+recordSpan(size) > capacity {
			continue
		}
		b := make([]byte, size)
		if _, err = f.ReadAt(b, off+recordHeader); err != nil {
			return fail(err)
		}
		if crc32.ChecksumIEEE(b) != binary.LittleEndian.Uint32(h[32:]) || !json.Valid(b) {
			continue
		}
		rec := queueRecord{off, binary.LittleEndian.Uint64(h[8:]), size, int64(binary.LittleEndian.Uint64(h[24:]))}
		q.records = append(q.records, rec)
		if rec.id > newest {
			newest = rec.id
			q.next = off + recordSpan(size)
		}
		off += recordSpan(size) - queueBlock
	}
	if newest > q.serial {
		q.serial = newest
	}
	sort.Slice(q.records, func(i, j int) bool { return q.records[i].id < q.records[j].id })
	return q, nil
}
func recordSpan(n int) int64 {
	return (int64(n) + recordHeader + queueBlock - 1) / queueBlock * queueBlock
}
func (q *diskQueue) Close() error   { return q.f.Close() }
func (q *diskQueue) Dropped() int64 { q.mu.Lock(); defer q.mu.Unlock(); return q.dropped }
func (q *diskQueue) persistCounter() error {
	q.serial++
	b := make([]byte, 20)
	binary.LittleEndian.PutUint64(b, q.serial)
	binary.LittleEndian.PutUint64(b[8:], uint64(q.dropped))
	binary.LittleEndian.PutUint32(b[16:], crc32.ChecksumIEEE(b[:16]))
	if _, err := q.f.WriteAt(b, 64+int64(q.serial%2)*64); err != nil {
		return err
	}
	return q.f.Sync()
}
func (q *diskQueue) erase(rec queueRecord) error {
	if _, err := q.f.WriteAt(make([]byte, recordHeader), rec.offset); err != nil {
		return err
	}
	return q.f.Sync()
}
func (q *diskQueue) push(b []byte, at int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	span := recordSpan(len(b))
	if len(b) == 0 || !json.Valid(b) || span > q.capacity-queueBlock {
		return fmt.Errorf("样本无效或超过缓存容量")
	}
	if q.next+span > q.capacity {
		q.next = queueBlock
	}
	// Invalidate and sync old headers BEFORE touching their payload. A crash may
	// lose an evicted record, but cannot make a torn replacement look committed.
	for i := 0; i < len(q.records); {
		r := q.records[i]
		overlap := r.offset < q.next+span && r.offset+recordSpan(r.size) > q.next
		if overlap || time.Now().UnixMilli()-r.at > int64(24*time.Hour/time.Millisecond) {
			q.dropped++
			if err := q.persistCounter(); err != nil {
				return err
			}
			if err := q.erase(r); err != nil {
				return err
			}
			q.records = append(q.records[:i], q.records[i+1:]...)
		} else {
			i++
		}
	}
	if err := q.persistCounter(); err != nil {
		return err
	}
	// Clear the target header, including any stale header from a failed attempt.
	if _, err := q.f.WriteAt(make([]byte, recordHeader), q.next); err != nil {
		return err
	}
	if err := q.f.Sync(); err != nil {
		return err
	}
	if _, err := q.f.WriteAt(b, q.next+recordHeader); err != nil {
		return err
	}
	if err := q.f.Sync(); err != nil {
		return err
	}
	h := make([]byte, recordHeader)
	copy(h, recordMagic)
	binary.LittleEndian.PutUint64(h[8:], q.serial)
	binary.LittleEndian.PutUint32(h[16:], uint32(len(b)))
	binary.LittleEndian.PutUint64(h[24:], uint64(at))
	binary.LittleEndian.PutUint32(h[32:], crc32.ChecksumIEEE(b))
	binary.LittleEndian.PutUint32(h[36:], crc32.ChecksumIEEE(h[:36]))
	if _, err := q.f.WriteAt(h, q.next); err != nil {
		return err
	}
	if err := q.f.Sync(); err != nil {
		return err
	}
	q.records = append(q.records, queueRecord{q.next, q.serial, len(b), at})
	q.next += span
	return nil
}
func (q *diskQueue) batch() []queueRecord {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []queueRecord
	for _, r := range q.records {
		if time.Now().UnixMilli()-r.at <= int64(24*time.Hour/time.Millisecond) {
			out = append(out, r)
		}
	}
	if len(out) > 1 {
		out = append([]queueRecord{out[len(out)-1]}, out[:len(out)-1]...)
	}
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}
func (q *diskQueue) read(rec queueRecord) ([]byte, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, r := range q.records {
		if r.id == rec.id {
			b := make([]byte, r.size)
			_, err := q.f.ReadAt(b, r.offset+recordHeader)
			return b, err
		}
	}
	return nil, nil // Evicted while a preceding HTTP request was in flight.
}
func (q *diskQueue) ack(rec queueRecord) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, r := range q.records {
		if r.id == rec.id {
			if err := q.erase(r); err != nil {
				return err
			}
			q.records = append(q.records[:i], q.records[i+1:]...)
			break
		}
	}
	return nil
}

// Migration runs before the uploader. Commit each record before unlinking its
// old file, and detect a previous committed copy if migration was interrupted.
func (q *diskQueue) migrate(dir string) error {
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(dir), "dropped")); err == nil {
		n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		if n > q.dropped {
			q.dropped = n
			if err = q.persistCounter(); err != nil {
				return err
			}
		}
	}
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	seen := make(map[[32]byte]bool)
	for _, rec := range q.records {
		b, e := q.read(rec)
		if e != nil {
			return e
		}
		seen[sha256.Sum256(b)] = true
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, f.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var s struct {
			At int64 `json:"at"`
		}
		if err = json.Unmarshal(b, &s); err != nil {
			return fmt.Errorf("旧缓存损坏 %s: %w", path, err)
		}
		key := sha256.Sum256(b)
		if !seen[key] {
			if err = q.push(b, s.At); err != nil {
				return err
			}
			seen[key] = true
		}
		if err = os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
