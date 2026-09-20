package hub

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Hub 同时提供网页与上报 API。mu 串行化接入码兑换、凭据更新、上报和清理，
// 使“校验凭据后写入”等多步操作不会与撤销操作交错；限速计数使用独立的 rateMu。
type Hub struct {
	DB     *sql.DB
	mu     sync.Mutex
	rateMu sync.Mutex
	rates  map[string]*rate
	quit   chan struct{}
}
type rate struct {
	Count int
	Reset time.Time
}
type user struct {
	Name string `json:"username"`
	Role string `json:"role"`
	CSRF string `json:"csrf"`
}

func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Open(path string) (*Hub, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	db, e := openStore(path)
	if e != nil {
		return nil, e
	}
	os.Chmod(path, 0600)
	h := &Hub{DB: db, rates: map[string]*rate{}, quit: make(chan struct{})}
	return h, nil
}
func (h *Hub) Close() { close(h.quit); h.DB.Close() }
