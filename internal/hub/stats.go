package hub

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func safeCSV(s string) string {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	startsWithFormula := strings.ContainsAny(trimmed[:min(1, len(trimmed))], "=+-@")
	startsWithControl := strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r")
	if startsWithFormula || startsWithControl {
		return "'" + s
	}
	return s
}
func (h *Hub) stats(w http.ResponseWriter, r *http.Request, export bool) {
	from, to, res := queryRange(r)
	q := r.URL.Query()
	rows, e := h.DB.Query(`
		SELECT
			g.node,
			g.uuid,
			g.name,
			COALESCE(r.user_id, ''),
			COALESCE(MAX(r.user_name), ''),
			COALESCE(SUM(r.occupied), 0),
			COALESCE(SUM(r.weighted), 0),
			COALESCE(SUM(r.occupancy_valid), 0),
			COALESCE(SUM(r.util_valid), 0),
			g.first_seen,
			COALESCE(SUM(r.attribution_valid), 0)
		FROM gpus AS g
		LEFT JOIN rollups AS r
		  ON g.node = r.node
		 AND g.uuid = r.uuid
		 AND r.res = ?
		 AND r.bucket >= ?
		 AND r.bucket < ?
		WHERE (? = '' OR g.node = ?)
		  AND (? = '' OR g.name = ?)
		  AND g.first_seen < ?
		GROUP BY g.node, g.uuid, r.user_id
		ORDER BY g.node, g.uuid, r.user_id
	`, res, from, to, q.Get("node"), q.Get("node"), q.Get("model"), q.Get("model"), to)
	if e != nil {
		fail(w, 500, "查询失败")
		return
	}
	out := []statRow{}
	for rows.Next() {
		var s statRow
		var first int64
		rows.Scan(
			&s.Node,
			&s.UUID,
			&s.Model,
			&s.UserID,
			&s.User,
			&s.Occupied,
			&s.Weighted,
			&s.OccupancyValid,
			&s.UtilValid,
			&first,
			&s.AttributionValid,
		)
		s.Occupied = max(0, s.Occupied/3600)
		s.Weighted = max(0, s.Weighted/3600)
		s.Expected = max(0, float64(min(to, time.Now().UnixMilli())-max(from, first))/1000)
		if s.Expected > 0 {
			s.Coverage = min(1, max(0, s.OccupancyValid/s.Expected))
			s.UtilCoverage = min(1, max(0, s.UtilValid/s.Expected))
		}
		out = append(out, s)
	}
	rows.Close()
	// 用户覆盖率取整张卡的“用户归属完整”时长，而非该用户恰好持卡的时长。
	// 未知进程会降低覆盖率，已知用户的持卡观测仍保留；不能把观测不完整误写成零占用。
	parent := map[string]statRow{}
	for _, s := range out {
		if s.UserID == "" {
			parent[s.Node+"/"+s.UUID] = s
		}
	}
	filtered := []statRow{}
	for _, s := range out {
		if s.UserID != "" {
			p := parent[s.Node+"/"+s.UUID]
			s.Coverage = 0
			if p.Expected > 0 {
				s.Coverage = min(1, max(0, p.AttributionValid/p.Expected))
			}
			s.OccupancyValid = p.AttributionValid
		}
		if user := q.Get("user"); user != "" && !strings.Contains(strings.ToLower(s.User+" "+s.UserID), strings.ToLower(user)) {
			continue
		}
		filtered = append(filtered, s)
	}
	if q.Get("view") == "users" {
		userStatsResponse(w, filtered, parent, export, from, to)
		return
	}
	if export {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="gpu-hours.csv"`)
		w.Write([]byte{0xEF, 0xBB, 0xBF})
		c := csv.NewWriter(w)
		c.Write([]string{
			"node",
			"gpu_uuid",
			"model",
			"user_id",
			"user",
			"occupied_hours",
			"weighted_hours",
			"occupancy_coverage",
			"utilization_coverage",
			"from_ms",
			"to_ms",
		})
		for _, s := range filtered {
			weighted, uc := fmt.Sprintf("%.6f", s.Weighted), fmt.Sprintf("%.4f", s.UtilCoverage)
			if s.UserID != "" {
				weighted = ""
				uc = ""
			}
			c.Write([]string{
				safeCSV(s.Node),
				safeCSV(s.UUID),
				safeCSV(s.Model),
				safeCSV(s.UserID),
				safeCSV(s.User),
				fmt.Sprintf("%.6f", s.Occupied),
				weighted,
				fmt.Sprintf("%.4f", s.Coverage),
				uc,
				strconv.FormatInt(from, 10),
				strconv.FormatInt(to, 10),
			})
		}
		c.Flush()
		return
	}
	respond(w, 200, map[string]any{"from": from, "to": to, "resolution": res, "rows": filtered})
}
