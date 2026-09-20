package agent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// pruneQueue bounds the disk queue and persists the cumulative dropped count.
func pruneQueue(dir string, dropped int64) int64 {
	// ReadDir 按文件名排序；文件名以采样时间开头，故容量不足时优先淘汰旧样本。
	// 时间或容量任一超限都触发淘汰，并把累计缺口数量带到后续快照中。
	files, _ := os.ReadDir(dir)
	var size int64
	for _, f := range files {
		if i, e := f.Info(); e == nil {
			size += i.Size()
		}
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		i, e := f.Info()
		if e != nil {
			continue
		}
		if size <= 100*1024*1024 && time.Since(i.ModTime()) <= 24*time.Hour {
			continue
		}
		if os.Remove(filepath.Join(dir, f.Name())) == nil {
			size -= i.Size()
			dropped++
			atomic(filepath.Join(configDir(), "dropped"), []byte(strconv.FormatInt(dropped, 10)))
		}
	}
	return dropped
}
