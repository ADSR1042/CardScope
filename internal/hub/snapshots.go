package hub

import (
	"gpu-monitor/internal/model"
	"log"
	"net/http"
	"strings"
)

func (h *Hub) snapshots(w http.ResponseWriter, r *http.Request) {
	credential := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(credential) != 64 {
		fail(w, 401, "节点凭据无效")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var id string
	if h.DB.QueryRow("SELECT id FROM nodes WHERE token_hash=?", hash(credential)).Scan(&id) != nil {
		fail(w, 401, "节点凭据无效")
		return
	}
	if !h.allowed("node/"+id, 600) {
		fail(w, 429, "上报过于频繁")
		return
	}
	var s model.Snapshot
	if e := decode(w, r, &s, 4<<20); e != nil || s.NodeID != id {
		fail(w, 400, "快照无效")
		return
	}
	if e := validate(s); e != nil {
		fail(w, 400, e.Error())
		return
	}
	if e := ingest(h.DB, s); e != nil {
		log.Print("snapshot persistence failed: ", e)
		fail(w, 500, "快照保存失败")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
	return
}
