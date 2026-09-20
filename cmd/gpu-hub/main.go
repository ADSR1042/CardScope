package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"golang.org/x/term"
	"gpu-monitor/internal/hub"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Println("gpu-hub serve --listen 0.0.0.0:8080 --data ./data")
		return
	}
	f := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := f.String("listen", "0.0.0.0:8080", "监听地址")
	data := f.String("data", "./data", "数据目录")
	f.Parse(os.Args[2:])
	h, e := hub.Open(filepath.Join(*data, "monitor.db"))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	defer h.Close()
	// 只在数据库尚未初始化时询问密码；正常重启沿用原有账号和节点凭据。
	if !h.Initialized() {
		reader := bufio.NewReader(os.Stdin)
		read := func(prompt string) string {
			fmt.Fprint(os.Stderr, prompt)
			if term.IsTerminal(int(os.Stdin.Fd())) {
				b, e := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if e != nil {
					return ""
				}
				return string(b)
			}
			s, _ := reader.ReadString('\n')
			return strings.TrimRight(s, "\r\n")
		}
		a := read("设置 admin 密码（10～72 字节）: ")
		v := read("设置 viewer 只读密码（10～72 字节）: ")
		if e = h.Initialize(a, v); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go h.Maintenance(ctx)
	srv := &http.Server{
		Addr:              *listen,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	go func() {
		<-ctx.Done()
		// 停止接受新请求，并给正在处理的请求最多 5 秒完成，随后释放数据库连接。
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(c)
	}()
	fmt.Println("GPU Monitor listening on http://" + *listen)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
