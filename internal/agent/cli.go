package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"gpu-monitor/internal/collect"
	"os"
	"time"
)

// Main runs the agent command line.
func Main() {
	if len(os.Args) < 2 {
		fmt.Println("gpu-agent doctor | setup --server http://IP:8080 | start | service install --user")
		return
	}
	var e error
	switch os.Args[1] {
	case "_worker":
		if len(os.Args) < 4 {
			os.Exit(2)
		}
		e = collect.Worker(os.Args[2], os.Args[3])
	case "doctor":
		c := collect.New()
		// CPU、网卡和磁盘速率依赖两次计数；首轮建立基线，第二轮才输出诊断结果。
		c.Sample(context.Background())
		time.Sleep(time.Second)
		s := c.Sample(context.Background())
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		e = enc.Encode(s)
	case "setup":
		e = setup(os.Args[2:])
	case "start":
		e = run()
	case "service":
		e = service(os.Args[2:])
	default:
		e = fmt.Errorf("unknown command")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
