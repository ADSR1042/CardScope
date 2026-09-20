package hub

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

func (h *Hub) Initialized() bool {
	var n int
	h.DB.QueryRow("SELECT count(*) FROM accounts").Scan(&n)
	return n >= 2
}
func (h *Hub) Initialize(admin, viewer string) error {
	if len(admin) < 10 || len(viewer) < 10 || len(admin) > 72 || len(viewer) > 72 {
		return fmt.Errorf("密码须为 10～72 字节")
	}
	if h.Initialized() {
		return fmt.Errorf("accounts already initialized")
	}
	a, e := bcrypt.GenerateFromPassword([]byte(admin), 12)
	if e != nil {
		return e
	}
	v, e := bcrypt.GenerateFromPassword([]byte(viewer), 12)
	if e != nil {
		return e
	}
	tx, e := h.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO accounts VALUES('admin',?,'admin'),('viewer',?,'viewer')", string(a), string(v)); e != nil {
		return e
	}
	return tx.Commit()
}

// allowed 是进程内的一分钟固定窗口限速器；key 分别按来源 IP 或已认证节点划分。
// 限制计数器数量，避免大量不同来源无限占用内存；服务重启后计数器会重置。
func (h *Hub) allowed(key string, limit int) bool {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	now := time.Now()
	if len(h.rates) > 4096 {
		for k, v := range h.rates {
			if now.After(v.Reset) {
				delete(h.rates, k)
			}
		}
		if len(h.rates) > 4096 {
			return false
		}
	}
	r := h.rates[key]
	if r == nil || now.After(r.Reset) {
		h.rates[key] = &rate{Count: 1, Reset: now.Add(time.Minute)}
		return true
	}
	r.Count++
	return r.Count <= limit
}

func (h *Hub) login(w http.ResponseWriter, r *http.Request, ip string) {
	if !h.allowed("login/"+ip, 10) {
		fail(w, 429, "尝试过于频繁，请稍后重试")
		return
	}
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decode(w, r, &b, 4096) != nil {
		fail(w, 400, "请求无效")
		return
	}
	var stored, role string
	err := h.DB.QueryRow("SELECT password,role FROM accounts WHERE username=?", b.Username).Scan(&stored, &role)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(stored), []byte(b.Password)) != nil {
		fail(w, 401, "账号或密码不正确")
		return
	}
	t, csrf := token(), token()
	if _, err = h.DB.Exec(
		"INSERT INTO sessions VALUES(?,?,?,?)",
		hash(t),
		b.Username,
		csrf,
		time.Now().Add(12*time.Hour).UnixMilli(),
	); err != nil {
		fail(w, 500, "会话创建失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "gpu_session",
		Value:    t,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
		MaxAge:   43200,
	})
	respond(w, 200, user{b.Username, role, csrf})
	return
}
