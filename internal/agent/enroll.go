package agent

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// setup 兑换一次性接入码；已有配置时拒绝覆盖，避免误换节点身份后混入旧缓存。
func setup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	server := fs.String("server", "", "中心 HTTP 地址")
	interval := fs.Int("interval", 5, "采样秒数 2..20")
	if e := fs.Parse(args); e != nil {
		return e
	}
	u, e := url.Parse(*server)
	invalidServer := e != nil ||
		u.Host == "" ||
		(u.Scheme != "http" && u.Scheme != "https") ||
		u.User != nil ||
		u.RawQuery != "" ||
		u.Fragment != ""
	if invalidServer {
		return fmt.Errorf("invalid server")
	}
	if *interval < 2 || *interval > 20 {
		return fmt.Errorf("采样间隔须为 2～20 秒")
	}
	if _, e := os.Stat(filepath.Join(configDir(), "config.json")); e == nil {
		return fmt.Errorf("已有配置；请先备份并移走 config.json，再重新接入")
	}
	code := secret("一次性接入码: ")
	var result struct {
		NodeID string `json:"node_id"`
		Token  string `json:"token"`
	}
	c := Config{Server: strings.TrimRight(*server, "/"), Interval: *interval}
	if e := post(c, "/enroll", map[string]string{"code": code}, &result); e != nil {
		return e
	}
	c.NodeID = result.NodeID
	c.Token = result.Token
	b, _ := json.MarshalIndent(c, "", "  ")
	if e := os.MkdirAll(configDir(), 0700); e != nil {
		return e
	}
	if e := atomic(filepath.Join(configDir(), "config.json"), b); e != nil {
		return e
	}
	fmt.Println("接入完成。运行 gpu-agent start 即可开始上报。")
	return nil
}
