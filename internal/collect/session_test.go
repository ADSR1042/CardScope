package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"gpu-monitor/internal/model"
)

func TestStreamReusesConnectionAndClosesOnEOF(t *testing.T) {
	opens, closes, calls := 0, 0, 0
	var output bytes.Buffer
	err := serveStream(strings.NewReader("{}\n{}\n{}\n"), &output, func() (func() (any, error), func()) {
		opens++
		return func() (any, error) { calls++; return calls, nil }, func() { closes++ }
	})
	if err != nil || opens != 1 || closes != 1 || calls != 3 || output.String() != "1\n2\n3\n" {
		t.Fatalf("open=%d close=%d calls=%d output=%q err=%v", opens, closes, calls, output.String(), err)
	}
}

// Subprocess fixture exercises the actual JSON pipes, timeouts, and reaping.
func TestSessionChild(t *testing.T) {
	if os.Getenv("CARDSCOPE_SESSION_CHILD") != "1" {
		return
	}
	dec, enc := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	seq := 0
	for {
		var req struct{}
		if err := dec.Decode(&req); err != nil {
			os.Exit(0)
		}
		seq++
		if os.Getenv("CARDSCOPE_SESSION_HANG") == "1" && seq > 1 {
			time.Sleep(time.Minute)
		}
		if err := enc.Encode(struct{ PID, Seq int }{os.Getpid(), seq}); err != nil {
			os.Exit(1)
		}
	}
}

func sessionFixture(hang bool) func() *exec.Cmd {
	return func() *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSessionChild$")
		cmd.Env = append(os.Environ(), "CARDSCOPE_SESSION_CHILD=1")
		if hang {
			cmd.Env = append(cmd.Env, "CARDSCOPE_SESSION_HANG=1")
		}
		return cmd
	}
}

func TestWorkerReuseTimeoutAndRecovery(t *testing.T) {
	starts := 0
	fixture := sessionFixture(true)
	w := &persistentWorker{command: func() *exec.Cmd { starts++; return fixture() }}
	defer w.close()
	var first, second struct{ PID, Seq int }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.sample(ctx, &first); err != nil {
		t.Fatal(err)
	}
	if first.PID == 0 || first.Seq != 1 {
		t.Fatal(first)
	}
	if got := w.budget(3 * time.Second); got != 3*time.Second {
		t.Fatal(got)
	}
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	started := time.Now()
	if err := w.sample(short, &second); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("timeout waited for child")
	}
	select {
	case <-w.done:
	case <-ctx.Done():
		t.Fatal("child not reaped")
	}
	if err := w.sample(ctx, &second); err != nil {
		t.Fatal(err)
	}
	if starts != 2 || second.PID == first.PID || second.Seq != 1 {
		t.Fatalf("%d %+v %+v", starts, first, second)
	}
	w.close()
	select {
	case <-w.done:
	case <-ctx.Done():
		t.Fatal("close did not reap child")
	}
}

func TestWorkerKeepsPIDAcrossSamples(t *testing.T) {
	w := &persistentWorker{command: sessionFixture(false)}
	defer w.close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pid := 0
	for i := 1; i <= 3; i++ {
		var r struct{ PID, Seq int }
		if err := w.sample(ctx, &r); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			pid = r.PID
		}
		if r.PID != pid || r.Seq != i {
			t.Fatalf("%+v", r)
		}
	}
}

func TestBlockedWorkerDoesNotSpawnReplacements(t *testing.T) {
	// Model a killed process still waiting in the kernel: Wait has not returned.
	w := &persistentWorker{cmd: &exec.Cmd{}, done: make(chan struct{}), stopping: true, command: func() *exec.Cmd { t.Fatal("spawned a replacement for blocked worker"); return nil }}
	for i := 0; i < 5; i++ {
		var out any
		if err := w.sample(context.Background(), &out); !errors.Is(err, errWorkerStopping) {
			t.Fatal(err)
		}
	}
}

func TestCancelledSampleDoesNotStartWorker(t *testing.T) {
	w := &persistentWorker{command: func() *exec.Cmd { t.Fatal("started after cancellation"); return nil }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out any
	if err := w.sample(ctx, &out); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStreamCleansUpAfterQueryFailure(t *testing.T) {
	closed := false
	err := serveStream(strings.NewReader("{}"), io.Discard, func() (func() (any, error), func()) {
		return func() (any, error) { return nil, io.ErrUnexpectedEOF }, func() { closed = true }
	})
	if !closed || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("closed=%v err=%v", closed, err)
	}
}

func TestGPUFailureSummary(t *testing.T) {
	for _, tc := range []struct {
		inventory string
		statuses  []string
		want      string
	}{
		{"ok", []string{"ok", "ok"}, "ok"},
		{"ok", []string{"timeout", "query_failed"}, "query_failed"},
		{"ok", []string{"ok", "gpu_lost"}, "partial"},
		{"inventory_failed", []string{"ok"}, "inventory_failed"},
		{"ok", nil, "ok"},
	} {
		var gpus []model.GPU
		for _, s := range tc.statuses {
			gpus = append(gpus, model.GPU{Status: s})
		}
		if got := gpuSummary(tc.inventory, gpus); got != tc.want {
			t.Fatalf("%+v: %s", tc, got)
		}
	}
}
