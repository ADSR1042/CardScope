package hub

import (
	"net/http"
	"time"
)

func (h *Hub) enroll(w http.ResponseWriter, r *http.Request, ip string) {
	if !h.allowed("enroll/"+ip, 30) {
		fail(w, 429, "尝试过于频繁")
		return
	}
	var b struct {
		Code string `json:"code"`
	}
	if decode(w, r, &b, 4096) != nil || len(b.Code) != 64 {
		fail(w, 400, "无效接入码")
		return
	}
	// 在同一锁内检查有效期并清空接入码，确保并发兑换最多只有一个请求成功。
	h.mu.Lock()
	defer h.mu.Unlock()
	var id string
	if h.DB.QueryRow("SELECT id FROM nodes WHERE code_hash=? AND code_expires>?", hash(b.Code), time.Now().UnixMilli()).Scan(&id) != nil {
		fail(w, 401, "接入码无效或已过期")
		return
	}
	t := token()
	if _, e := h.DB.Exec("UPDATE nodes SET token_hash=?,code_hash='',code_expires=0 WHERE id=?", hash(t), id); e != nil {
		fail(w, 500, "接入失败")
		return
	}
	respond(w, 200, map[string]string{"node_id": id, "token": t})
	return
}
