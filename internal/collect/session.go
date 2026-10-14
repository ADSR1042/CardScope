package collect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

var errWorkerStopping = errors.New("previous GPU worker has not exited")

// One request is in flight per slot. Collector.Sample is called serially.
// The process stays alive between samples and owns its NVML session.
type persistentWorker struct {
	kind, arg       string
	cmd             *exec.Cmd
	input           io.WriteCloser
	output          io.ReadCloser
	encoder         *json.Encoder
	decoder         *json.Decoder
	done            chan struct{}
	stopping, ready bool
	command         func() *exec.Cmd // injectable process boundary for regression tests
}

func (w *persistentWorker) budget(warm time.Duration) time.Duration {
	if w.done != nil {
		select {
		case <-w.done:
			return 15 * time.Second
		default:
		}
	}
	if !w.ready {
		return 15 * time.Second
	}
	return warm
}

func (w *persistentWorker) start() error {
	var cmd *exec.Cmd
	if w.command != nil {
		cmd = w.command()
	} else {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		cmd = exec.Command(exe, "_worker", w.kind+"-stream", w.arg)
	}
	configureWorker(cmd)
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return err
	}
	w.cmd, w.input, w.output = cmd, input, output
	w.encoder, w.decoder = json.NewEncoder(input), json.NewDecoder(output)
	w.done = make(chan struct{})
	w.stopping, w.ready = false, false
	done := w.done
	go func() { cmd.Wait(); close(done) }()
	return nil
}

func (w *persistentWorker) sample(ctx context.Context, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.cmd != nil {
		select {
		case <-w.done:
			w.input.Close()
			w.output.Close()
			w.cmd = nil
			w.ready = false
		default:
			if w.stopping {
				return errWorkerStopping
			}
		}
	}
	if w.cmd == nil {
		if err := w.start(); err != nil {
			return err
		}
	}
	// Capture the pipes, not the mutable slot. A killed, blocked process may
	// finish later; it must never consume a replacement worker's response.
	encoder, decoder := w.encoder, w.decoder
	type reply struct {
		data json.RawMessage
		err  error
	}
	result := make(chan reply, 1)
	go func() {
		var data json.RawMessage
		err := encoder.Encode(struct{}{})
		if err == nil {
			err = decoder.Decode(&data)
		}
		result <- reply{data, err}
	}()
	select {
	case <-ctx.Done():
		w.close()
		return ctx.Err()
	case r := <-result:
		if err := ctx.Err(); err != nil {
			w.close()
			return err
		}
		if r.err != nil {
			w.close()
			return r.err
		}
		if err := json.Unmarshal(r.data, out); err != nil {
			w.close()
			return err
		}
		w.ready = true
		return nil
	}
}

// Do not synchronously Wait after SIGKILL: an uninterruptible driver ioctl
// may not exit. Retain that slot and refuse to spawn another until it does.
func (w *persistentWorker) close() {
	if w == nil || w.cmd == nil {
		return
	}
	w.stopping, w.ready = true, false
	w.cmd.Process.Kill()
	w.input.Close()
	w.output.Close()
}

func fallbackStreamSampler(kind, arg string) func() (any, error) {
	return func() (any, error) {
		if kind == "inventory" {
			return Inventory()
		}
		return GPU(arg), nil
	}
}

func streamWorker(kind, arg string) error {
	if kind != "gpu" && kind != "inventory" {
		return fmt.Errorf("unknown stream worker")
	}
	return serveStream(os.Stdin, os.Stdout, func() (func() (any, error), func()) {
		return newStreamSampler(kind, arg)
	})
}

func serveStream(input io.Reader, output io.Writer, open func() (func() (any, error), func())) error {
	dec, enc := json.NewDecoder(input), json.NewEncoder(output)
	var sample func() (any, error)
	for {
		var request struct{}
		if err := dec.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if sample == nil {
			var cleanup func()
			sample, cleanup = open()
			defer cleanup()
		}
		value, err := sample()
		if err != nil {
			return err
		}
		if err := enc.Encode(value); err != nil {
			return err
		}
	}
}
