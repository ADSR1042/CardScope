package hub

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
)

type userStat struct {
	Node     string    `json:"node"`
	UID      string    `json:"uid"`
	User     string    `json:"user"`
	Hours    float64   `json:"hours"`
	Cards    int       `json:"cards"`
	Coverage float64   `json:"coverage"`
	Details  []statRow `json:"details"`
}

// 汇总始终以节点和 UID 为身份；同名用户不会跨节点合并。
func groupUserStats(rows []statRow) []userStat {
	groups := map[string]*userStat{}
	for _, s := range rows {
		if s.UserID == "" {
			continue
		}
		key := s.Node + "/" + s.UserID
		g := groups[key]
		if g == nil {
			g = &userStat{Node: s.Node, UID: s.UserID, User: s.User, Details: []statRow{}}
			groups[key] = g
		}
		g.Hours += s.Occupied
		if s.Occupied > 0 {
			g.Cards++
		}
		g.Details = append(g.Details, s)
	}
	out := []userStat{}
	for _, g := range groups {
		var valid, expected float64
		for _, s := range g.Details {
			valid += s.OccupancyValid
			expected += s.Expected
		}
		if expected > 0 {
			g.Coverage = min(1, max(0, valid/expected))
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hours != out[j].Hours {
			return out[i].Hours > out[j].Hours
		}
		return out[i].Node+out[i].UID < out[j].Node+out[j].UID
	})
	return out
}

func userStatsResponse(w http.ResponseWriter, rows []statRow, parents map[string]statRow, export bool, from, to int64) {
	groups := groupUserStats(rows)
	if !export {
		incomplete := false
		for _, p := range parents {
			if p.Expected > 0 && p.AttributionValid/p.Expected < .99 {
				incomplete = true
			}
		}
		respond(w, 200, map[string]any{"users": groups, "incomplete": incomplete, "from": from, "to": to})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="user-gpu-hours.csv"`)
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	c := csv.NewWriter(w)
	c.Write([]string{
		"用户",
		"节点 ID",
		"UID",
		"占用卡时",
		"使用卡数",
		"用户归属覆盖率",
		"开始时间（毫秒）",
		"结束时间（毫秒）",
	})
	for _, g := range groups {
		c.Write([]string{
			safeCSV(g.User),
			safeCSV(g.Node),
			safeCSV(g.UID),
			fmt.Sprintf("%.6f", g.Hours),
			strconv.Itoa(g.Cards),
			fmt.Sprintf("%.4f", g.Coverage),
			strconv.FormatInt(from, 10),
			strconv.FormatInt(to, 10),
		})
	}
	c.Flush()
}
