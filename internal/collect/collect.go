// Package collect 负责普通用户权限下的只读采集，不执行设备控制或提权操作。
package collect

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	netinfo "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"gpu-monitor/internal/model"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

func smi(args ...string) ([][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	exe := "nvidia-smi"
	if _, e := exec.LookPath(exe); e != nil {
		exe = "/usr/lib/wsl/lib/nvidia-smi"
	}
	b, e := exec.CommandContext(ctx, exe, args...).Output()
	if e != nil {
		return nil, e
	}
	return csv.NewReader(strings.NewReader(string(b))).ReadAll()
}

// parsed 将 nvidia-smi 的数值转换为统一单位；N/A 等非数值保持缺失，不转换成零。
func parsed(v string, scale float64) *float64 {
	x, e := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if e != nil || x < 0 {
		return nil
	}
	return model.Number(x * scale)
}

// Inventory 优先通过 NVML 枚举显卡，失败或结果为空时尝试 nvidia-smi。
func Inventory() ([]model.GPU, error) {
	if g, e := nvInventory(); e == nil && len(g) > 0 {
		return g, nil
	}
	rows, e := smi("--query-gpu=uuid,name,index,pci.bus_id", "--format=csv,noheader,nounits")
	if e != nil {
		return nil, e
	}
	out := []model.GPU{}
	for _, r := range rows {
		if len(r) == 4 {
			i, _ := strconv.Atoi(strings.TrimSpace(r[2]))
			out = append(out, model.GPU{
				UUID:  strings.TrimSpace(r[0]),
				Name:  strings.TrimSpace(r[1]),
				Index: i,
				PCI:   strings.TrimSpace(r[3]),
			})
		}
	}
	return out, nil
}

// GPU 查询单张显卡。仅当 NVML 返回整体查询错误时回退到 nvidia-smi；
// NVML 成功但某个指标不受支持时保留该字段状态，不把缺失值补成零。
func GPU(uuid string) model.GPU {
	g, e := nvGPU(uuid)
	if e == nil {
		return g
	}
	g = model.GPU{
		UUID:          uuid,
		Status:        "query_failed",
		Fields:        map[string]string{"nvml": e.Error()},
		Source:        "nvidia-smi",
		Processes:     []model.Process{},
		ProcessStatus: "query_failed",
	}
	if strings.Contains(e.Error(), "gpu_lost") {
		g.Status = "gpu_lost"
	}
	if strings.Contains(e.Error(), "permission_denied") {
		g.Status = "permission_denied"
	}
	rows, e := smi(
		"-i",
		uuid,
		"--query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,pci.bus_id",
		"--format=csv,noheader,nounits",
	)
	if e != nil || len(rows) != 1 || len(rows[0]) != 7 {
		return g
	}
	r := rows[0]
	g.Name = strings.TrimSpace(r[0])
	g.Util = parsed(r[1], 1)
	g.MemoryUsed = parsed(r[2], 1048576)
	g.MemoryTotal = parsed(r[3], 1048576)
	g.Temperature = parsed(r[4], 1)
	g.Power = parsed(r[5], 1)
	g.PCI = strings.TrimSpace(r[6])
	g.Status = "ok"
	fields := map[string]*float64{
		"util":        g.Util,
		"memory":      g.MemoryUsed,
		"temperature": g.Temperature,
		"power":       g.Power,
	}
	for k, p := range fields {
		g.Fields[k] = "ok"
		if p == nil {
			g.Fields[k] = "unsupported"
		}
	}
	rows, e = smi(
		"-i",
		uuid,
		"--query-compute-apps=pid,used_gpu_memory,process_name",
		"--format=csv,noheader,nounits",
	)
	if e == nil {
		g.ProcessStatus = "ok"
		for _, r := range rows {
			if len(r) != 3 {
				continue
			}
			pid, e := strconv.ParseInt(strings.TrimSpace(r[0]), 10, 32)
			if e == nil {
				g.Processes = append(g.Processes, model.Process{
					PID:    int32(pid),
					Memory: parsed(r[1], 1048576),
					Name:   filepath.Base(strings.TrimSpace(r[2])),
				})
			}
		}
	}
	return g
}

// Worker 是内部 _worker 子命令入口，每次只执行一个查询并将 JSON 写到标准输出。
func Worker(kind, arg string) error {
	enc := json.NewEncoder(os.Stdout)
	switch kind {
	case "inventory":
		g, e := Inventory()
		if e != nil {
			return e
		}
		return enc.Encode(g)
	case "gpu":
		return enc.Encode(GPU(arg))
	case "disk":
		return enc.Encode(diskUsage(arg))
	}
	return fmt.Errorf("unknown worker")
}

// 容量查询错误作为字段状态上报，避免把权限不足误报成超时。
func diskUsage(path string) model.Disk {
	r := model.Disk{Path: path, Status: "query_failed"}
	d, err := disk.Usage(path)
	if err == nil {
		r.Total, r.Free, r.Status = d.Total, d.Free, "ok"
	} else if os.IsPermission(err) {
		r.Status = "permission_denied"
	}
	return r
}

// 同一路径可能叠加挂载；只查询一次当前可见路径，再合并设备的重复挂载。
func uniqueMounts(parts []disk.PartitionStat) []disk.PartitionStat {
	last := map[string]int{}
	for i, p := range parts {
		last[p.Mountpoint] = i
	}
	devices := map[string]bool{}
	result := []disk.PartitionStat{}
	for i, p := range parts {
		skip := last[p.Mountpoint] != i ||
			devices[p.Device] ||
			p.Fstype == "overlay" ||
			p.Device == "none" ||
			strings.HasPrefix(p.Mountpoint, "/snap/")
		if skip {
			continue
		}
		devices[p.Device] = true
		result = append(result, p)
	}
	return result
}

// worker 复用当前可执行文件启动短生命周期子进程。
// 将可能卡住的驱动调用隔离到进程中，父进程可在截止时间到达后发起终止。
func worker(ctx context.Context, kind, arg string, out any) error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	cmd := exec.CommandContext(ctx, exe, "_worker", kind, arg)
	b, e := cmd.Output()
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}

// Collector 保留上轮计数、硬件清单和进程首次观测时间。
// Sample 会更新这些状态，调用方应串行采样，不要共享给多个采样循环。
type Collector struct {
	inventory   []model.GPU
	inventoryAt time.Time
	previous    time.Time
	nets        map[string]netinfo.IOCountersStat
	ios         map[string]disk.IOCountersStat
	seen        map[string]int64
	WSL         bool
}

func New() *Collector {
	b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return &Collector{
		nets: map[string]netinfo.IOCountersStat{},
		ios:  map[string]disk.IOCountersStat{},
		seen: map[string]int64{},
		WSL:  strings.Contains(strings.ToLower(string(b)), "microsoft"),
	}
}

// Sample 采集一轮系统与 GPU 数据，失败信息与成功指标一起返回，允许部分结果可用。
func (c *Collector) Sample(ctx context.Context) model.Snapshot {
	now := time.Now()
	s := model.Snapshot{
		Version:   1,
		At:        now.UnixMilli(),
		Interval:  5,
		GPUs:      []model.GPU{},
		GPUStatus: "ok",
	}
	s.Hostname, _ = os.Hostname()
	s.System = model.System{
		Cores:    runtime.NumCPU(),
		WSL:      c.WSL,
		Errors:   map[string]string{},
		Networks: []model.Network{},
		Disks:    []model.Disk{},
		IO:       []model.IO{},
	}
	// GPU 查询与系统采集并行，各自使用截止时间，避免一张异常显卡拖住其他采集项。
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		gctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		c.gpus(gctx, &s)
	}()
	if v, e := cpu.Percent(0, false); e == nil && len(v) > 0 {
		if !c.previous.IsZero() {
			s.System.CPU = model.Number(v[0])
		}
	} else {
		s.System.Errors["cpu"] = "unavailable"
	}
	if v, e := load.Avg(); e == nil {
		s.System.Load = model.Number(v.Load1)
	} else {
		s.System.Errors["load"] = "unavailable"
	}
	if v, e := mem.VirtualMemory(); e == nil {
		s.System.MemoryTotal = v.Total
		s.System.MemoryAvailable = v.Available
	} else {
		s.System.Errors["memory"] = "unavailable"
	}
	if v, e := mem.SwapMemory(); e == nil {
		s.System.SwapTotal = v.Total
		s.System.SwapUsed = v.Used
	}
	dt := now.Sub(c.previous).Seconds()
	valid := !c.previous.IsZero() && dt > 0
	ns, e := netinfo.IOCounters(true)
	if e != nil {
		s.System.Errors["network"] = "unavailable"
	}
	ifs, _ := netinfo.Interfaces()
	up := map[string]bool{}
	for _, i := range ifs {
		for _, f := range i.Flags {
			if f == "up" {
				up[i.Name] = true
			}
		}
	}
	primary := ""
	if b, e := os.ReadFile("/proc/net/route"); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) > 2 && f[1] == "00000000" {
				primary = f[0]
				break
			}
		}
	}
	nextnets := map[string]netinfo.IOCountersStat{}
	// 首次采样或计数回退时不计算速率；后者可能由接口重建或计数器重置造成。
	for _, n := range ns {
		r := model.Network{Name: n.Name, RX: n.BytesRecv, TX: n.BytesSent, Up: up[n.Name], Primary: n.Name == primary}
		if p, ok := c.nets[n.Name]; ok && valid && n.BytesRecv >= p.BytesRecv && n.BytesSent >= p.BytesSent {
			r.RXRate = model.Number(float64(n.BytesRecv-p.BytesRecv) / dt)
			r.TXRate = model.Number(float64(n.BytesSent-p.BytesSent) / dt)
		}
		s.System.Networks = append(s.System.Networks, r)
		nextnets[n.Name] = n
	}
	c.nets = nextnets
	ios, e := disk.IOCounters()
	if e != nil {
		s.System.Errors["disk_io"] = "unavailable"
	}
	for name, n := range ios {
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		r := model.IO{Device: name}
		if p, ok := c.ios[name]; ok && valid && n.ReadBytes >= p.ReadBytes && n.WriteBytes >= p.WriteBytes {
			r.ReadRate = model.Number(float64(n.ReadBytes-p.ReadBytes) / dt)
			r.WriteRate = model.Number(float64(n.WriteBytes-p.WriteBytes) / dt)
		}
		s.System.IO = append(s.System.IO, r)
	}
	c.ios = ios
	// 挂载点容量查询共用 1 秒预算，每个子进程最多 250 毫秒，避免慢挂载累计拖长采样。
	diskCtx, diskCancel := context.WithTimeout(ctx, time.Second)
	defer diskCancel()
	parts, e := disk.Partitions(false)
	if e != nil {
		s.System.Errors["disk"] = "unavailable"
	}
	for _, p := range uniqueMounts(parts) {
		if len(s.System.Disks) >= 32 {
			break
		}
		d := model.Disk{Path: p.Mountpoint, Device: p.Device, Status: "query_failed"}
		dctx, cancel := context.WithTimeout(diskCtx, 250*time.Millisecond)
		var result model.Disk
		if worker(dctx, "disk", p.Mountpoint, &result) == nil {
			d.Total = result.Total
			d.Free = result.Free
			d.Status = result.Status
		} else if dctx.Err() != nil {
			d.Status = "timeout"
		}
		cancel()
		s.System.Disks = append(s.System.Disks, d)
	}
	wg.Wait()
	c.previous = now
	return s
}
func (c *Collector) gpus(ctx context.Context, s *model.Snapshot) {
	// 硬件枚举缓存 60 秒；枚举失败时继续尝试已知显卡，不把暂时失败当成卡已移除。
	if time.Since(c.inventoryAt) > 60*time.Second {
		var list []model.GPU
		dctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		e := worker(dctx, "inventory", "", &list)
		cancel()
		if e == nil {
			c.inventory = list
			c.inventoryAt = time.Now()
		} else {
			s.GPUStatus = "inventory_failed"
		}
	}
	result := make([]model.GPU, len(c.inventory))
	// 限制同时运行的查询进程数；每个协程只写自己的结果槽位，等待结束后再统一归属用户。
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, base := range c.inventory {
		wg.Add(1)
		go func(i int, base model.GPU) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				result[i] = model.GPU{
					UUID:          base.UUID,
					Name:          base.Name,
					Index:         base.Index,
					Status:        "timeout",
					ProcessStatus: "unknown",
					Fields:        map[string]string{},
				}
				return
			}
			g := base
			dctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if e := worker(dctx, "gpu", base.UUID, &g); e != nil {
				g = base
				g.Status = "query_failed"
				g.ProcessStatus = "unknown"
				g.Fields = map[string]string{"query": "timeout_or_failure"}
			}
			g.Index = base.Index
			g.SeenAt = s.At
			if g.Name == "" {
				g.Name = base.Name
			}
			result[i] = g
		}(i, base)
	}
	wg.Wait()
	nextseen := map[string]int64{}
	for i := range result {
		g := &result[i]
		if g.Fields == nil {
			g.Fields = map[string]string{}
		}
		if c.WSL {
			// WSL 的 GPU 可能与 Windows 共享，返回的 PID 不能直接当作 Linux PID 解析。
			g.ProcessStatus = "wsl_limited"
			g.Fields["scope"] = "WSL GPU shared with Windows; process attribution unavailable"
		}
		for j := range g.Processes {
			p := &g.Processes[j]
			p.User = "未知"
			if !c.WSL {
				if pr, e := process.NewProcess(p.PID); e == nil {
					p.Created, _ = pr.CreateTime()
					if ids, e := pr.Uids(); e == nil && len(ids) > 0 {
						p.UID = strconv.Itoa(int(ids[0]))
						p.User = p.UID
						if u, e := pr.Username(); e == nil {
							p.User = u
						}
					}
					if name, e := pr.Name(); e == nil {
						p.Name = name
					}
				}
			}
			if strings.Contains(strings.ToLower(p.Name), "mps") {
				g.ProcessStatus = "mps_unsupported"
			}
			// PID 会重用；加入显卡 UUID 和可读取的创建时间，避免将新进程接到旧记录上。
			key := fmt.Sprintf("%s/%d/%d", g.UUID, p.PID, p.Created)
			p.FirstSeen = c.seen[key]
			if p.FirstSeen == 0 {
				p.FirstSeen = s.At
			}
			p.LastSeen = s.At
			nextseen[key] = p.FirstSeen
		}
		if g.ProcessStatus == "ok" {
			for _, p := range g.Processes {
				if p.UID == "" {
					g.Fields["users"] = "partial"
				}
			}
		}
	}
	c.seen = nextseen
	s.GPUs = result
}
