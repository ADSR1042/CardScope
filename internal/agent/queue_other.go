//go:build !linux

package agent

import "os"

// Non-Linux is used by unit tests; production agents run on Linux.
func reserveQueue(f *os.File, size int64) error {
	b := make([]byte, 1024*1024)
	for off := int64(0); off < size; {
		n := int64(len(b))
		if size-off < n {
			n = size - off
		}
		if _, e := f.WriteAt(b[:n], off); e != nil {
			return e
		}
		off += n
	}
	return f.Sync()
}
func syncQueueDirectory(string) error { return nil }

func ensureQueueReservation(*os.File, int64) error { return nil }
