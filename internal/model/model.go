// Package model 定义客户端与中心服务共用的上报结构。
// 时间戳统一使用 Unix 毫秒，容量使用字节，速率使用字节/秒。
// 可缺失指标使用指针：nil 表示不可用，指向 0 则表示成功读到了零值。
package model

// Process 是一次采样中观测到的 GPU 计算进程。
// Created 是系统进程创建时间；FirstSeen/LastSeen 是采集器的观测时间，不能当作任务起止时间。
type Process struct {
	PID       int32    `json:"pid"`
	UID       string   `json:"uid"`
	User      string   `json:"user"`
	Name      string   `json:"name"`
	Created   int64    `json:"created"`
	Memory    *float64 `json:"memory"`
	FirstSeen int64    `json:"first_seen"`
	LastSeen  int64    `json:"last_seen"`
}

// GPU 以 UUID 关联历史，Index 只用于显示，重启后可能改变。
// Status 描述整卡查询状态，Fields 和 ProcessStatus 分别描述单项指标与进程列表的可用性。
type GPU struct {
	UUID          string            `json:"uuid"`
	Name          string            `json:"name"`
	Index         int               `json:"index"`
	PCI           string            `json:"pci"`
	Util          *float64          `json:"util"`
	MemoryUsed    *float64          `json:"memory_used"`
	MemoryTotal   *float64          `json:"memory_total"`
	Temperature   *float64          `json:"temperature"`
	Power         *float64          `json:"power"`
	ECC           *float64          `json:"ecc"`
	Status        string            `json:"status"`
	Fields        map[string]string `json:"fields"`
	Processes     []Process         `json:"processes"`
	ProcessStatus string            `json:"process_status"`
	Source        string            `json:"source"`
	SeenAt        int64             `json:"seen_at"`
}

// Network 保留每个接口的原始计数与差分速率，不在采集端汇总，避免上下层网卡重复计数。
type Network struct {
	Name    string   `json:"name"`
	RX      uint64   `json:"rx"`
	TX      uint64   `json:"tx"`
	RXRate  *float64 `json:"rx_rate"`
	TXRate  *float64 `json:"tx_rate"`
	Up      bool     `json:"up"`
	Primary bool     `json:"primary"`
}

// Disk 描述一个挂载点的容量与采集状态。
type Disk struct {
	Path   string `json:"path"`
	Device string `json:"device"`
	Total  uint64 `json:"total"`
	Free   uint64 `json:"free"`
	Status string `json:"status"`
}

// IO 描述块设备在当前采样区间内的读写速率。
type IO struct {
	Device    string   `json:"device"`
	ReadRate  *float64 `json:"read_rate"`
	WriteRate *float64 `json:"write_rate"`
}

// System 汇总一次采样中的主机资源指标。
type System struct {
	CPU             *float64          `json:"cpu"`
	Cores           int               `json:"cores"`
	Load            *float64          `json:"load"`
	MemoryTotal     uint64            `json:"memory_total"`
	MemoryAvailable uint64            `json:"memory_available"`
	SwapTotal       uint64            `json:"swap_total"`
	SwapUsed        uint64            `json:"swap_used"`
	Networks        []Network         `json:"networks"`
	Disks           []Disk            `json:"disks"`
	IO              []IO              `json:"io"`
	WSL             bool              `json:"wsl"`
	Errors          map[string]string `json:"errors"`
}

// Snapshot 是一次完整上报；NodeID + BootID + Seq 构成中心端的幂等键。
// BootID 每次客户端启动重新生成，并非操作系统启动 ID；跨启动不连续计算卡时。
// Backfill 标记历史补传，中心端据此避免用旧数据刷新在线状态。
type Snapshot struct {
	Version      int    `json:"version"`
	NodeID       string `json:"node_id"`
	BootID       string `json:"boot_id"`
	Seq          int64  `json:"seq"`
	At           int64  `json:"at"`
	Interval     int    `json:"interval"`
	Hostname     string `json:"hostname"`
	System       System `json:"system"`
	GPUs         []GPU  `json:"gpus"`
	GPUStatus    string `json:"gpu_status"`
	CacheDropped int64  `json:"cache_dropped"`
	Backfill     bool   `json:"backfill"`
}

// Number 返回数值的指针，便于构造带可选指标的上报结构。
func Number(v float64) *float64 { return &v }

// Idle 只判断指标是否满足“疑似空闲”；调用方还须检查节点在线、数据新鲜。
// 进程列表不可读、利用率缺失或显存仍被大量占用时，都不能仅凭“没看到进程”判断空闲。
func Idle(g GPU) bool {
	if g.Status != "ok" || g.ProcessStatus != "ok" || len(g.Processes) != 0 {
		return false
	}
	if g.Util == nil || *g.Util >= 5 {
		return false
	}
	if g.MemoryUsed == nil || g.MemoryTotal == nil || *g.MemoryTotal <= 0 {
		return false
	}

	return *g.MemoryUsed / *g.MemoryTotal < 0.05
}
