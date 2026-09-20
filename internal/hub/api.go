package hub

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// api 先处理登录、接入码兑换与节点上报，再进入网页会话认证。
// 两条认证路径刻意分开，避免节点的 Bearer 凭据获得面板读取或管理权限。

func (h *Hub) api(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if path == "/login-copy" && r.Method == "GET" {
		h.loginCopy(w, r)
		return
	}
	if r.Method != "GET" && !sameOrigin(r) {
		fail(w, 403, "来源不允许")
		return
	}
	if path == "/login" && r.Method == "POST" {
		h.login(w, r, ip)
		return
	}
	if path == "/enroll" && r.Method == "POST" {
		h.enroll(w, r, ip)
		return
	}
	if path == "/snapshots" && r.Method == "POST" {
		h.snapshots(w, r)
		return
	}
	u, e := h.session(r)
	if e != nil {
		fail(w, 401, "请先登录")
		return
	}
	if r.Method != "GET" {
		csrf := r.Header.Get("X-CSRF-Token")
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(u.CSRF)) != 1 {
			fail(w, 403, "会话校验失败")
			return
		}
	}
	if path == "/me" && r.Method == "GET" {
		respond(w, 200, u)
		return
	}
	if path == "/logout" && r.Method == "POST" {
		c, _ := r.Cookie("gpu_session")
		h.DB.Exec("DELETE FROM sessions WHERE hash=?", hash(c.Value))
		http.SetCookie(w, &http.Cookie{Name: "gpu_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if path == "/nodes" && r.Method == "GET" {
		h.nodes(w)
		return
	}
	if path == "/history" && r.Method == "GET" {
		h.history(w, r)
		return
	}
	if path == "/cluster/history" && r.Method == "GET" {
		h.clusterHistory(w, r)
		return
	}
	if path == "/stats" && r.Method == "GET" {
		h.stats(w, r, false)
		return
	}
	if path == "/export" && r.Method == "GET" {
		h.stats(w, r, true)
		return
	}
	if path == "/events" && r.Method == "GET" {
		h.events(w, r)
		return
	}
	if u.Role != "admin" {
		fail(w, 403, "仅管理员可修改")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if path == "/login-copy" && r.Method == "PUT" {
		h.loginCopy(w, r)
		return
	}
	if path == "/nodes" && r.Method == "POST" {
		h.createNode(w, r)
		return
	}
	if strings.HasPrefix(path, "/nodes/") {
		h.updateNode(w, r, path)
		return
	}
	if path == "/accounts/password" && r.Method == "POST" {
		h.changePassword(w, r)
		return
	}
	fail(w, 404, "接口不存在")
}
