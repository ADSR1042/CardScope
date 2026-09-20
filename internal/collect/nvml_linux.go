//go:build linux

package collect

import (
	"fmt"
	"github.com/ebitengine/purego"
	"gpu-monitor/internal/model"
	"strings"
	"unsafe"
)

// nvml 保存运行时加载的函数入口，无需客户端安装 CUDA Toolkit 或 C 编译器。
// 函数签名和下方结构体布局必须与对应版本的 NVML C ABI 一致。
type nvml struct {
	lib     uintptr
	init    func() int32
	close   func() int32
	count   func(*uint32) int32
	byIndex func(uint32, *uintptr) int32
	byUUID  func(string, *uintptr) int32
	name    func(uintptr, *byte, uint32) int32
	uuid    func(uintptr, *byte, uint32) int32
}

// bind 把缺少符号时的绑定 panic 转为错误，使旧驱动缺少可选查询时可以降级。
func bind[T any](lib uintptr, name string) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("symbol %s unavailable", name)
		}
	}()
	purego.RegisterLibFunc(&out, lib, name)
	return
}

// openNVML 先走系统动态库搜索路径，再尝试 WSL 提供的驱动库位置。
func openNVML() (*nvml, error) {
	var lib uintptr
	var err error
	for _, p := range []string{"libnvidia-ml.so.1", "/usr/lib/wsl/lib/libnvidia-ml.so.1"} {
		lib, err = purego.Dlopen(p, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	n := &nvml{lib: lib}
	n.init, err = bind[func() int32](lib, "nvmlInit_v2")
	if err != nil {
		return nil, err
	}
	if rc := n.init(); rc != 0 {
		return nil, fmt.Errorf("NVML init: %s", nvStatus(rc))
	}
	n.close, _ = bind[func() int32](lib, "nvmlShutdown")
	n.count, _ = bind[func(*uint32) int32](lib, "nvmlDeviceGetCount_v2")
	n.byIndex, _ = bind[func(uint32, *uintptr) int32](lib, "nvmlDeviceGetHandleByIndex_v2")
	n.byUUID, _ = bind[func(string, *uintptr) int32](lib, "nvmlDeviceGetHandleByUUID")
	n.name, _ = bind[func(uintptr, *byte, uint32) int32](lib, "nvmlDeviceGetName")
	n.uuid, _ = bind[func(uintptr, *byte, uint32) int32](lib, "nvmlDeviceGetUUID")
	return n, nil
}
func nvStatus(rc int32) string {
	switch rc {
	case 0:
		return "ok"
	case 3:
		return "unsupported"
	case 4:
		return "permission_denied"
	case 6:
		return "not_found"
	case 15:
		return "gpu_lost"
	default:
		return fmt.Sprintf("nvml_error_%d", rc)
	}
}
func nvText(f func(uintptr, *byte, uint32) int32, h uintptr) string {
	if f == nil {
		return ""
	}
	b := make([]byte, 128)
	if f(h, &b[0], 128) != 0 {
		return ""
	}
	return strings.TrimRight(string(b), "\x00")
}
func nvInventory() ([]model.GPU, error) {
	n, e := openNVML()
	if e != nil {
		return nil, e
	}
	defer n.close()
	var count uint32
	if n.count == nil || n.byIndex == nil {
		return nil, fmt.Errorf("NVML missing enumeration")
	}
	if rc := n.count(&count); rc != 0 {
		return nil, fmt.Errorf("%s", nvStatus(rc))
	}
	if count > 128 {
		return nil, fmt.Errorf("too many GPUs")
	}
	out := []model.GPU{}
	for i := uint32(0); i < count; i++ {
		var h uintptr
		if n.byIndex(i, &h) != 0 {
			continue
		}
		u := nvText(n.uuid, h)
		if u != "" {
			out = append(out, model.GPU{UUID: u, Index: int(i), Name: nvText(n.name, h)})
		}
	}
	return out, nil
}
func nvGPU(uuid string) (model.GPU, error) {
	g := model.GPU{
		UUID:          uuid,
		Status:        "ok",
		Source:        "nvml",
		Fields:        map[string]string{},
		Processes:     []model.Process{},
		ProcessStatus: "unsupported",
	}
	n, e := openNVML()
	if e != nil {
		return g, e
	}
	defer n.close()
	var h uintptr
	if n.byUUID == nil {
		return g, fmt.Errorf("missing UUID lookup")
	}
	if rc := n.byUUID(uuid, &h); rc != 0 {
		return g, fmt.Errorf("%s", nvStatus(rc))
	}
	g.Name = nvText(n.name, h)
	// PCI v2 的旧式总线 ID 位于结构体前 16 字节；缓冲区额外留空，容纳整个 C 结构体。
	if f, e := bind[func(uintptr, unsafe.Pointer) int32](n.lib, "nvmlDeviceGetPciInfo_v2"); e == nil {
		var pci [256]byte
		if f(h, unsafe.Pointer(&pci[0])) == 0 {
			g.PCI = strings.TrimRight(string(pci[:16]), "\x00")
		}
	}
	// 每个标量查询分别记录返回状态；例如功耗不支持，不影响显存和温度的有效性。
	read := func(symbol, key string, extra *uint32, scale float64) *float64 {
		var v uint32
		rc := int32(3)
		if extra == nil {
			f, e := bind[func(uintptr, *uint32) int32](n.lib, symbol)
			if e == nil {
				rc = f(h, &v)
			}
		} else {
			f, e := bind[func(uintptr, uint32, *uint32) int32](n.lib, symbol)
			if e == nil {
				rc = f(h, *extra, &v)
			}
		}
		g.Fields[key] = nvStatus(rc)
		if rc == 15 {
			g.Status = "gpu_lost"
		}
		if rc != 0 {
			return nil
		}
		return model.Number(float64(v) / scale)
	}
	var temp uint32
	g.Temperature = read("nvmlDeviceGetTemperature", "temperature", &temp, 1)
	g.Power = read("nvmlDeviceGetPowerUsage", "power", nil, 1000)
	var util struct{ GPU, Memory uint32 }
	if f, e := bind[func(uintptr, unsafe.Pointer) int32](n.lib, "nvmlDeviceGetUtilizationRates"); e == nil {
		rc := f(h, unsafe.Pointer(&util))
		g.Fields["util"] = nvStatus(rc)
		if rc == 0 {
			g.Util = model.Number(float64(util.GPU))
		}
	} else {
		g.Fields["util"] = "unsupported"
	}
	var mem struct{ Total, Free, Used uint64 }
	if f, e := bind[func(uintptr, unsafe.Pointer) int32](n.lib, "nvmlDeviceGetMemoryInfo"); e == nil {
		rc := f(h, unsafe.Pointer(&mem))
		g.Fields["memory"] = nvStatus(rc)
		if rc == 0 {
			g.MemoryTotal = model.Number(float64(mem.Total))
			g.MemoryUsed = model.Number(float64(mem.Used))
		}
	} else {
		g.Fields["memory"] = "unsupported"
	}
	var ecc uint64
	if f, e := bind[func(uintptr, uint32, uint32, *uint64) int32](n.lib, "nvmlDeviceGetTotalEccErrors"); e == nil {
		rc := f(h, 1, 1, &ecc)
		g.Fields["ecc"] = nvStatus(rc)
		if rc == 0 {
			g.ECC = model.Number(float64(ecc))
			if ecc > 0 {
				g.Fields["health"] = "ecc_errors_observed"
			}
		}
	}
	var cur, pending uint32
	mig := false
	if f, e := bind[func(uintptr, *uint32, *uint32) int32](n.lib, "nvmlDeviceGetMigMode"); e == nil && f(h, &cur, &pending) == 0 && cur == 1 {
		mig = true
		g.Fields["mig"] = "enabled"
	}
	// 无版本后缀的进程查询使用 v1 ABI；显式 Pad 对齐后面的 uint64，不能随意调整字段顺序。
	type processV1 struct {
		PID    uint32
		Pad    uint32
		Memory uint64
	}
	arr := make([]processV1, 4096)
	count := uint32(len(arr))
	if f, e := bind[func(uintptr, *uint32, unsafe.Pointer) int32](n.lib, "nvmlDeviceGetComputeRunningProcesses"); e == nil {
		rc := f(h, &count, unsafe.Pointer(&arr[0]))
		g.ProcessStatus = nvStatus(rc)
		if rc == 0 && count <= 4096 {
			for _, p := range arr[:count] {
				var m *float64
				// NVML 用全 1 的 uint64 表示显存不可读，该哨兵值不能展示成实际显存用量。
				if p.Memory != ^uint64(0) {
					m = model.Number(float64(p.Memory))
				}
				g.Processes = append(g.Processes, model.Process{PID: int32(p.PID), Memory: m})
			}
		}
	}
	if mig {
		g.ProcessStatus = "mig_unsupported"
	}
	return g, nil
}
