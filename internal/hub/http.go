package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

// decode 只接受一个有大小上限的 JSON 对象，并拒绝未知字段和尾随内容。
func decode(w http.ResponseWriter, r *http.Request, v any, max int64) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return fmt.Errorf("JSON required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("extra data")
	}
	return nil
}

// sameOrigin 检查浏览器提供的 Origin；命令行客户端通常没有该头，所以允许缺省。
// 它不是身份认证的替代品，网页写操作仍须验证会话和 CSRF，节点上报仍须验证令牌。
