package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// service 仅安装当前用户的 systemd 服务，不提权、不启用 linger。
// 服务使用程序的绝对路径，因此安装后移动可执行文件需要重新安装服务。

func service(args []string) error {
	if runtime.GOOS != "linux" || len(args) != 2 || args[0] != "install" || args[1] != "--user" {
		return fmt.Errorf("usage: gpu-agent service install --user (Linux)")
	}
	exe, _ := os.Executable()
	if strings.ContainsAny(exe, "\n\r%\"") {
		return fmt.Errorf("unsupported executable path")
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config/systemd/user")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	cfg := configDir()
	if strings.ContainsAny(cfg, "\n\r%\"") {
		return fmt.Errorf("unsupported config path")
	}
	unit := fmt.Sprintf(`[Unit]
Description=GPU Monitor Agent
After=network.target

[Service]
ExecStart="%s" start
Environment="GPU_AGENT_HOME=%s"
Restart=on-failure
RestartSec=10
UMask=0077
NoNewPrivileges=yes

[Install]
WantedBy=default.target
`, exe, cfg)
	if e := atomic(filepath.Join(dir, "gpu-agent.service"), []byte(unit)); e != nil {
		return e
	}
	for _, a := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", "--now", "gpu-agent.service"}} {
		if b, e := exec.Command("systemctl", a...).CombinedOutput(); e != nil {
			return fmt.Errorf("%s: %w", b, e)
		}
	}
	fmt.Println("用户服务已启动。未修改 linger；登出后或重启后的常驻取决于管理员设置。")
	return nil
}
