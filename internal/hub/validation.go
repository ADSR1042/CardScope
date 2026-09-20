package hub

import (
	"fmt"
	"gpu-monitor/internal/model"
	"math"
	"time"
)

// validate 在落库前限制快照规模、数值和时间范围，防止异常数据污染统计。
// 接受最近 25 小时覆盖客户端 24 小时缓存，并给未来时钟保留 30 秒容差。

func validate(s model.Snapshot) error {
	now := time.Now().UnixMilli()
	invalidSnapshot := s.Version != 1 ||
		s.NodeID == "" || len(s.NodeID) > 100 ||
		s.BootID == "" || len(s.BootID) > 100 ||
		s.Seq < 1 ||
		s.Interval < 2 || s.Interval > 60 ||
		s.At > now+30000 || s.At < now-25*3600000 ||
		len(s.Hostname) > 200 ||
		len(s.GPUs) > 128 ||
		len(s.System.Networks) > 256 ||
		len(s.System.Disks) > 64 ||
		len(s.System.IO) > 256
	if invalidSnapshot {
		return fmt.Errorf("invalid snapshot or clock outside accepted window")
	}
	check := func(p *float64, max float64) bool {
		return p == nil || (!math.IsNaN(*p) && !math.IsInf(*p, 0) && *p >= 0 && *p <= max)
	}
	if !check(s.System.CPU, 100) || !check(s.System.Load, 1e6) || s.System.MemoryAvailable > s.System.MemoryTotal {
		return fmt.Errorf("invalid system metrics")
	}
	for _, n := range s.System.Networks {
		if len(n.Name) > 128 || !check(n.RXRate, 1e15) || !check(n.TXRate, 1e15) {
			return fmt.Errorf("invalid network")
		}
	}
	for _, d := range s.System.Disks {
		if len(d.Path) > 2048 || len(d.Device) > 512 || d.Free > d.Total {
			return fmt.Errorf("invalid disk")
		}
	}
	for _, d := range s.System.IO {
		if len(d.Device) > 256 || !check(d.ReadRate, 1e15) || !check(d.WriteRate, 1e15) {
			return fmt.Errorf("invalid IO")
		}
	}
	seen := map[string]bool{}
	for _, g := range s.GPUs {
		if g.UUID == "" || len(g.UUID) > 128 || len(g.Name) > 200 || seen[g.UUID] || len(g.Processes) > 4096 || len(g.Fields) > 64 {
			return fmt.Errorf("invalid GPU")
		}
		seen[g.UUID] = true
		for k, v := range g.Fields {
			if len(k) > 64 || len(v) > 512 {
				return fmt.Errorf("invalid field")
			}
		}
		invalidMetrics := !check(g.Util, 100) ||
			!check(g.MemoryUsed, 1e15) ||
			!check(g.MemoryTotal, 1e15) ||
			!check(g.Temperature, 250) ||
			!check(g.Power, 1e5)
		if invalidMetrics {
			return fmt.Errorf("invalid GPU metric")
		}
		for _, p := range g.Processes {
			if p.PID <= 0 || len(p.Name) > 256 || len(p.User) > 256 || len(p.UID) > 64 || !check(p.Memory, 1e15) {
				return fmt.Errorf("invalid process")
			}
		}
	}
	return nil
}
