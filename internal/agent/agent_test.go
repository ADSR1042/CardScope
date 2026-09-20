package agent

import (
	"context"
	"gpu-monitor/internal/model"
	"testing"
)

func TestCancelledSamplingIsNotQueued(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, complete := sampleWhileRunning(ctx, func(context.Context) model.Snapshot {
		cancel() // 模拟首次枚举进行中收到 SIGTERM。
		return model.Snapshot{GPUStatus: "inventory_failed"}
	})
	if complete {
		t.Fatal("cancelled partial sample would reach queue")
	}
	called := false
	_, complete = sampleWhileRunning(ctx, func(context.Context) model.Snapshot { called = true; return model.Snapshot{} })
	if complete || called {
		t.Fatal("sampling continued after shutdown")
	}
	_, complete = sampleWhileRunning(
		context.Background(),
		func(context.Context) model.Snapshot {
			return model.Snapshot{GPUStatus: "inventory_failed"}
		},
	)
	if !complete {
		t.Fatal("ordinary inventory failure must still report heartbeat")
	}
}
