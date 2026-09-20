package agent

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/term"
	"os"
	"path/filepath"
	"strings"
)

// Config 只保存连接与采样配置，Token 是只能上报本节点的凭据。
// 配置通过交互式 setup 创建，以 0600 权限保存在用户目录中。
type Config struct {
	Server   string `json:"server"`
	NodeID   string `json:"node_id"`
	Token    string `json:"token"`
	Interval int    `json:"interval"`
}

func configDir() string {
	if s := os.Getenv("GPU_AGENT_HOME"); s != "" {
		return s
	}
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "gpu-monitor")
}
func secret(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, e := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if e != nil {
			panic(e)
		}
		return strings.TrimSpace(string(b))
	}
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}
func random() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}

func load() (Config, error) {
	var c Config
	b, e := os.ReadFile(filepath.Join(configDir(), "config.json"))
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	if e == nil && (c.Token == "" || c.NodeID == "") {
		e = errors.New("invalid config")
	}
	if e == nil && (c.Interval < 2 || c.Interval > 20) {
		e = errors.New("采样间隔须为 2～20 秒，请修改 config.json 中的 interval")
	}
	return c, e
}

// atomic 在同目录写临时文件再重命名，使读取方不会读到只写了一部分的配置或样本。
// 这保证文件替换的原子可见性，不代表已经执行断电持久化所需的 fsync。
func atomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if e := os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	if e := os.Chmod(tmp, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
