package hub

import (
	"gpu-monitor/internal/model"
	"net/http"
	"strconv"
	"time"
)

func queryRange(r *http.Request) (int64, int64, int64) {
	now := time.Now().UnixMilli()
	from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	to, _ := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	if to <= 0 || to > now {
		to = now
	}
	if from <= 0 {
		from = to - 24*3600000
	}
	from = max(from, now-500*86400000)
	if from >= to {
		from = to - 3600000
	}
	res := int64(60000)
	if from < now-30*86400000 || to-from > 7*86400000 {
		res = 3600000
	}
	return from / res * res, (to + res - 1) / res * res, res
}
func (h *Hub) history(w http.ResponseWriter, r *http.Request) {
	from, to, res := queryRange(r)
	q := r.URL.Query()
	rows, e := h.DB.Query(`
		SELECT bucket, weighted, util_valid, memory_sum, memory_valid
		FROM rollups
		WHERE node = ?
		  AND uuid = ?
		  AND user_id = ''
		  AND res = ?
		  AND bucket >= ?
		  AND bucket < ?
		ORDER BY bucket
	`, q.Get("node"), q.Get("gpu"), res, from, to)
	if e != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []any{}
	previousBucket := int64(-1)
	for rows.Next() {
		var t int64
		var weighted, valid, mem, mv float64
		rows.Scan(&t, &weighted, &valid, &mem, &mv)
		// 显式补出缺失时间桶的 null，防止前端把两段有效曲线直接连过数据空档。
		if previousBucket >= 0 {
			for missing := previousBucket + res; missing < t; missing += res {
				out = append(out, map[string]any{"at": missing, "util": nil, "memory": nil, "coverage": 0})
			}
		}
		previousBucket = t
		var util, memory *float64
		if valid > 0 {
			util = model.Number(weighted / valid * 100)
		}
		if mv > 0 {
			memory = model.Number(mem / mv)
		}
		out = append(out, map[string]any{"at": t, "util": util, "memory": memory, "coverage": valid / (float64(res) / 1000)})
	}
	respond(w, 200, map[string]any{"from": from, "to": to, "resolution": res, "points": out})
}

type statRow struct {
	Node             string  `json:"node"`
	UUID             string  `json:"uuid"`
	Model            string  `json:"model"`
	UserID           string  `json:"user_id"`
	User             string  `json:"user"`
	Occupied         float64 `json:"occupied_hours"`
	Weighted         float64 `json:"weighted_hours"`
	OccupancyValid   float64 `json:"occupancy_seconds"`
	UtilValid        float64 `json:"util_seconds"`
	Expected         float64 `json:"expected_seconds"`
	Coverage         float64 `json:"coverage"`
	UtilCoverage     float64 `json:"util_coverage"`
	AttributionValid float64 `json:"attribution_seconds"`
}

// safeCSV 将可能被电子表格解释为公式的文本标记为字面量；普通 CSV 引号不足以阻止公式执行。
