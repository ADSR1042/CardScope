package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func queueFile(t *testing.T, dir, name string, size int64, age time.Duration) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(size); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-age)
	if err = os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestQueueRetention(t *testing.T) {
	for _, capacity := range []bool{false, true} {
		name := "age"
		if capacity {
			name = "capacity"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("GPU_AGENT_HOME", home)
			dir := filepath.Join(home, "queue")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			size, age := int64(10), 25*time.Hour
			if capacity {
				size, age = 60*1024*1024, time.Hour
			}
			old := queueFile(t, dir, "001.json", size, age)
			newest := queueFile(t, dir, "002.json", size, 0)
			if got := pruneQueue(dir, 7); got != 8 {
				t.Fatalf("dropped = %d, want 8", got)
			}
			if _, err := os.Stat(old); !os.IsNotExist(err) {
				t.Fatalf("old sample remains: %v", err)
			}
			if _, err := os.Stat(newest); err != nil {
				t.Fatalf("new sample lost: %v", err)
			}
			b, err := os.ReadFile(filepath.Join(home, "dropped"))
			if err != nil || string(b) != "8" {
				t.Fatalf("persisted dropped = %q, %v", b, err)
			}
			if got := pruneQueue(dir, 8); got != 8 {
				t.Fatalf("second prune counted another loss: %d", got)
			}
		})
	}
}
