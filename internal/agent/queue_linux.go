//go:build linux

package agent

import (
	"golang.org/x/sys/unix"
	"os"
)

// Do not silently substitute truncate: it creates a sparse file, reserving no
// disk blocks. Unsupported filesystems must report failure to the operator.
func reserveQueue(f *os.File, size int64) error { return unix.Fallocate(int(f.Fd()), 0, 0, size) }
func syncQueueDirectory(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}

func ensureQueueReservation(f *os.File, size int64) error { return reserveQueue(f, size) }
