//go:build linux

package agent

import (
	"fmt"
	"os"
	"syscall"
)

// acquireLock 用内核文件锁防止同一配置启动多个采集器，避免重复统计。
// 锁随文件描述符关闭或进程退出释放；磁盘上的锁文件残留不意味着仍被占用。
func acquireLock(path string) (*os.File, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("此配置已有客户端运行，不能重复启动")
	}
	return f, nil
}
