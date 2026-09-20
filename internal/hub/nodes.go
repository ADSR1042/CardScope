package hub

import (
	"encoding/json"
	"gpu-monitor/internal/model"
	"net/http"
	"time"
)

func (h *Hub) nodes(w http.ResponseWriter) {
	rows, e := h.DB.Query(`
		SELECT id, name, expected, networks, snapshot, sample_at, received_at, token_hash, created, tags
		FROM nodes
		ORDER BY name
	`)
	if e != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, name, nets, raw, tokenHash, rawTags string
		var expected int
		var at, received, created int64
		rows.Scan(&id, &name, &expected, &nets, &raw, &at, &received, &tokenHash, &created, &rawTags)
		tags := []string{}
		json.Unmarshal([]byte(rawTags), &tags)
		var snapshot model.Snapshot
		json.Unmarshal([]byte(raw), &snapshot)
		var networks []string
		json.Unmarshal([]byte(nets), &networks)
		out = append(out, map[string]any{
			"id":          id,
			"name":        name,
			"expected":    expected,
			"networks":    networks,
			"tags":        tags,
			"snapshot":    snapshot,
			"sample_at":   at,
			"received_at": received,
			"online":      time.Now().UnixMilli()-received <= 30000 && tokenHash != "",
			"enrolled":    tokenHash != "",
			"created":     created,
		})
	}
	respond(w, 200, out)
}

// queryRange 按可用保留期选择分钟或小时精度，并把查询范围对齐到桶边界。
// 返回值就是 API 实际使用的范围，可能略宽于用户提交的起止时间。
