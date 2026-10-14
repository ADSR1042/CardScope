//go:build linux

package collect

import (
	"os/exec"
	"syscall"
)

func configureWorker(cmd *exec.Cmd) {
	// Also stop persistent workers if the agent is killed without cleanup.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
