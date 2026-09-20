package collect

import (
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
	"os"
	"path/filepath"
	"testing"
)

func TestHostMetricsAndCurrentProcess(t *testing.T) {
	cores, err := cpu.Counts(true)
	if err != nil || cores <= 0 {
		t.Fatalf("CPU count: %d %v", cores, err)
	}
	memory, err := mem.VirtualMemory()
	if err != nil {
		t.Fatal(err)
	}
	if memory.Total == 0 || memory.Available > memory.Total {
		t.Fatalf("invalid memory counters: %+v", memory)
	}
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	created, err := p.CreateTime()
	if err != nil || created <= 0 {
		t.Fatalf("process identity timestamp: %d %v", created, err)
	}
}

func TestMountRefreshAndStacking(t *testing.T) {
	a := disk.PartitionStat{Device: "/dev/sdb1", Mountpoint: "/mnt/data", Fstype: "ext4"}
	b := disk.PartitionStat{Device: "/dev/sdc1", Mountpoint: "/mnt/data", Fstype: "ext4"}
	got := uniqueMounts([]disk.PartitionStat{a, b})
	if len(got) != 1 || got[0].Device != b.Device {
		t.Fatalf("stacked mounts: %v", got)
	}
	got = uniqueMounts([]disk.PartitionStat{a})
	if len(got) != 1 || got[0].Device != a.Device {
		t.Fatalf("unmount did not reveal old mount: %v", got)
	}
	if len(uniqueMounts(nil)) != 0 {
		t.Fatal("removed mounts retained")
	}
	alias := a
	alias.Mountpoint = "/mnt/alias"
	if len(uniqueMounts([]disk.PartitionStat{a, alias})) != 1 {
		t.Fatal("device aliases duplicated")
	}
}

func TestDiskUsageFailureIsNotTimeout(t *testing.T) {
	if d := diskUsage(filepath.Join(t.TempDir(), "missing")); d.Status != "query_failed" {
		t.Fatalf("%+v", d)
	}
	if d := diskUsage(t.TempDir()); d.Status != "ok" || d.Total == 0 {
		t.Fatalf("%+v", d)
	}
}

func TestUnavailableSmiValues(t *testing.T) {
	for _, v := range []string{"[N/A]", "Not Supported", "", "-1"} {
		if parsed(v, 1) != nil {
			t.Fatal(v)
		}
	}
	p := parsed(" 12.5 ", 1048576)
	if p == nil || *p != 12.5*1048576 {
		t.Fatal("unit conversion")
	}
}
