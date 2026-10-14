//go:build !linux

package collect

import "os/exec"

func configureWorker(cmd *exec.Cmd) {}
