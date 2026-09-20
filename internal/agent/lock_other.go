//go:build !linux

package agent

import (
	"fmt"
	"os"
)

func acquireLock(path string) (*os.File, error) {
	return nil, fmt.Errorf("客户端常驻运行仅支持 Linux")
}
