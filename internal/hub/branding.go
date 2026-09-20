package hub

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"unicode/utf8"
)

// LoginCopy 仅包含公开展示的纯文本，不允许在这里存放密钥或 HTML。
type LoginCopy struct {
	Eyebrow     string `json:"eyebrow"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Caption     string `json:"caption"`
}

func defaultLoginCopy() LoginCopy {
	return LoginCopy{"CARDSCOPE", "让算力\n清晰可见。", "每台服务器，每张显卡。\n一个安静、专注的资源工作空间。", "实时状态 · 用户归属 · 使用统计"}
}

func (h *Hub) loginCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		v := defaultLoginCopy()
		var raw string
		err := h.DB.QueryRow("SELECT value FROM settings WHERE key='login_copy'").Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			fail(w, 500, "读取登录页设置失败")
			return
		}
		if err == nil && json.Unmarshal([]byte(raw), &v) != nil {
			fail(w, 500, "登录页设置无效")
			return
		}
		respond(w, 200, v)
		return
	}
	// 写入入口在 api 中通过管理员角色和 CSRF 校验后才调用。
	var v LoginCopy
	if decode(w, r, &v, 16384) != nil {
		fail(w, 400, "文案过长或格式无效")
		return
	}
	tooLong := utf8.RuneCountInString(v.Eyebrow) > 80 ||
		utf8.RuneCountInString(v.Title) > 120 ||
		utf8.RuneCountInString(v.Description) > 1000 ||
		utf8.RuneCountInString(v.Caption) > 200
	if tooLong {
		fail(w, 400, "文案过长或格式无效")
		return
	}
	if _, err := h.DB.Exec(`
		INSERT INTO settings(key, value) VALUES ('login_copy', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value
	`, marshal(v)); err != nil {
		fail(w, 500, "保存登录页设置失败")
		return
	}
	respond(w, 200, v)
}
