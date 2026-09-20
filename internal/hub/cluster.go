package hub

import (
	"gpu-monitor/internal/model"
	"net/http"
	"time"
)

// clusterHistory averages only valid GPU-seconds, never treating missing data as idle.
// A single grouped query replaces one history request per GPU on the homepage.
func (h *Hub) clusterHistory(w http.ResponseWriter, r *http.Request) {
	const resolution int64 = 15 * 60000
	to := time.Now().UnixMilli() / 60000 * 60000
	from := (to - 24*3600000) / resolution * resolution
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT (bucket / ?) * ?, SUM(weighted), SUM(util_valid)
		FROM rollups
		WHERE res = 60000 AND user_id = '' AND bucket >= ? AND bucket < ?
		AND node IN (SELECT id FROM nodes)
		GROUP BY (bucket / ?) ORDER BY (bucket / ?)
	`, resolution, resolution, from, to, resolution, resolution)
	if err != nil {
		fail(w, 500, "集群趋势查询失败")
		return
	}
	defer rows.Close()
	type point struct {
		At   int64    `json:"at"`
		Util *float64 `json:"util"`
	}
	values := map[int64]*float64{}
	for rows.Next() {
		var at int64
		var weighted, valid float64
		if err = rows.Scan(&at, &weighted, &valid); err != nil {
			fail(w, 500, "集群趋势查询失败")
			return
		}
		if valid > 0 {
			values[at] = model.Number(weighted / valid * 100)
		}
	}
	if rows.Err() != nil {
		fail(w, 500, "集群趋势查询失败")
		return
	}
	points := []point{}
	for at := from; at < to; at += resolution {
		points = append(points, point{At: at, Util: values[at]})
	}
	respond(w, 200, map[string]any{"from": from, "to": to, "resolution": resolution, "points": points})
}
