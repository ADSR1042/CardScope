package hub

import (
	"encoding/json"
	"net/http"
)

func (h *Hub) events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// 旧的正常/进程变动事件不再展示；保留历史失败记录。
	rows, e := h.DB.Query(`
		SELECT at, kind, detail
		FROM events
		WHERE node = ?
		  AND uuid = ?
		  AND (
			kind IN ('started', 'failure', 'missing')
			OR (kind = 'state' AND json_extract(detail, '$.status') != 'ok')
		  )
		ORDER BY at DESC
		LIMIT 100
	`, q.Get("node"), q.Get("gpu"))
	if e != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var at int64
		var kind, detail string
		rows.Scan(&at, &kind, &detail)
		var d any
		json.Unmarshal([]byte(detail), &d)
		out = append(out, map[string]any{"at": at, "kind": kind, "detail": d})
	}
	respond(w, 200, out)
}
