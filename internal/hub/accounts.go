package hub

import (
	"golang.org/x/crypto/bcrypt"
	"net/http"
)

func (h *Hub) changePassword(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decode(w, r, &b, 4096) != nil || (b.Username != "admin" && b.Username != "viewer") || len(b.Password) < 10 || len(b.Password) > 72 {
		fail(w, 400, "密码须为 10～72 字节")
		return
	}
	p, e := bcrypt.GenerateFromPassword([]byte(b.Password), 12)
	if e != nil {
		fail(w, 500, "更新失败")
		return
	}
	tx, e := h.DB.Begin()
	if e != nil {
		fail(w, 500, "更新失败")
		return
	}
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE accounts SET password=? WHERE username=?", string(p), b.Username); e == nil {
		_, e = tx.Exec("DELETE FROM sessions WHERE username=?", b.Username)
	}
	if e != nil || tx.Commit() != nil {
		fail(w, 500, "更新失败")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
	return
}
