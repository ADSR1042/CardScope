package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"gpu-monitor/internal/collect"
	"gpu-monitor/internal/model"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// run 将采样与网络上传分开：主循环先写本地队列，上传协程成功后才删除样本。
// 中心端按样本身份去重，因此响应丢失后重试也不会重复累计 GPU-hours。

func run() error {
	c, e := load()
	if e != nil {
		return e
	}
	lock, e := acquireLock(filepath.Join(configDir(), "agent.lock"))
	if e != nil {
		return e
	}
	defer lock.Close()
	q, e := openQueue(filepath.Join(configDir(), "queue.bin"), queueCapacity)
	if e != nil {
		return e
	}
	defer q.Close()
	if e = q.migrate(filepath.Join(configDir(), "queue")); e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	col := collect.New()
	boot := random()
	seq := int64(0)
	// 单个上传协程顺序发送；网络超时不会阻塞主循环继续采样和写入队列。
	done := make(chan struct{})
	go func() {
		defer close(done)
		uploadLoop(ctx, c, q)
	}()
	ticker := time.NewTicker(time.Duration(c.Interval) * time.Second)
	defer ticker.Stop()
	defer func() { stop(); <-done }()
	fmt.Println("采集已启动；仅主动上报，不监听入站端口。")
	for {
		seq++
		s, complete := sampleWhileRunning(ctx, col.Sample)
		if !complete {
			return nil
		}
		s.NodeID = c.NodeID
		s.BootID = boot
		s.Seq = seq
		s.Interval = c.Interval
		s.CacheDropped = q.Dropped()
		b, _ := json.Marshal(s)
		if e := q.push(b, s.At); e != nil {
			fmt.Fprintln(os.Stderr, "缓存写入失败，本次样本未保存:", e)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// 停机取消可能打断首次显卡枚举；这样的半成品不能进入补传队列。
// 取消发生在采样完成之后时，本轮已完成的数据仍是有效观测。
func sampleWhileRunning(ctx context.Context, sample func(context.Context) model.Snapshot) (model.Snapshot, bool) {
	if ctx.Err() != nil {
		return model.Snapshot{}, false
	}
	s := sample(ctx)
	return s, ctx.Err() == nil
}
