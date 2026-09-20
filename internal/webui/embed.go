// Package webui embeds the frontend built by Vite.
package webui

import (
	"embed"
	"io/fs"
)

// README.txt 保证尚未构建前端时也有可嵌入文件；dist 由 Vite 生成。
//
//go:embed assets/*
var site embed.FS

// Assets returns the frontend files without their build directory prefix.
func Assets() (fs.FS, error) { return fs.Sub(site, "assets/dist") }
