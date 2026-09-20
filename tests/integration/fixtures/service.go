//go:build ignore

// Harmless service double for installer tests: no network or GPU access.
package main

import (
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		path := filepath.Join(os.Args[len(os.Args)-1], "monitor.db")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte("synthetic database"), 0600); err != nil {
				panic(err)
			}
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		path := filepath.Join(os.Getenv("GPU_AGENT_HOME"), "config.json")
		if err := os.WriteFile(path, []byte(`{"node_id":"synthetic","token":"synthetic","interval":5}`), 0600); err != nil {
			panic(err)
		}
		return
	}
	for {
		time.Sleep(time.Second)
	}
}
