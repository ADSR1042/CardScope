package collect

import (
	"github.com/shirou/gopsutil/v4/disk"
	"path/filepath"
	"testing"
)

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
