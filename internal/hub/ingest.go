package hub

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gpu-monitor/internal/model"
	"sort"
	"strings"
	"time"
)

// ingest 将幂等写入、相邻积分修正、事件保存和实时状态更新放在一个事务中。
// 补传可能乱序到达，因此必须按采样时间查询邻居，不能依赖接收顺序计算卡时。

func ingest(db *sql.DB, s model.Snapshot) error {
	if e := validate(s); e != nil {
		return e
	}
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := tx.Exec("INSERT OR IGNORE INTO samples VALUES(?,?,?,?,?)", s.NodeID, s.BootID, s.Seq, s.At, marshal(s.System))
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		// 重试命中同一个样本身份时直接返回，不重复写事件或积分，也不刷新在线时间。
		return tx.Commit()
	}
	var oldRaw string
	var oldAt int64
	if e = tx.QueryRow("SELECT snapshot,sample_at FROM nodes WHERE id=?", s.NodeID).Scan(&oldRaw, &oldAt); e != nil {
		return e
	}
	var old model.Snapshot
	json.Unmarshal([]byte(oldRaw), &old)
	// 枚举失败只能说明本轮无法查询。保留已知卡的身份，但不沿用旧的实时指标。
	// 写入无效观测点，使后续卡时积分也能看到此次采集缺口。
	if s.GPUStatus != "ok" {
		present := map[string]bool{}
		for _, g := range s.GPUs {
			present[g.UUID] = true
		}
		for _, g := range old.GPUs {
			if present[g.UUID] {
				continue
			}
			var firstSeen int64
			if e = tx.QueryRow("SELECT first_seen FROM gpus WHERE node=? AND uuid=?", s.NodeID, g.UUID).Scan(&firstSeen); e != nil {
				return e
			}
			if firstSeen > s.At {
				continue
			}
			g.Status = "query_failed"
			g.ProcessStatus = "unknown"
			g.Processes = nil
			g.Util, g.MemoryUsed, g.MemoryTotal, g.Temperature, g.Power, g.ECC = nil, nil, nil, nil, nil, nil
			g.Fields = map[string]string{"inventory": s.GPUStatus}
			s.GPUs = append(s.GPUs, g)
		}
	}
	for _, g := range s.GPUs {
		p := point{
			At:           s.At,
			Seq:          s.Seq,
			Interval:     s.Interval,
			Model:        g.Name,
			Users:        map[string]string{},
			Status:       g.Status,
			ProcessValid: g.Status == "ok" && g.ProcessStatus == "ok",
		}
		p.UserValid = p.ProcessValid
		if g.Status == "ok" {
			p.Util = g.Util
			p.Memory = g.MemoryUsed
		}
		p.Occupied = len(g.Processes) > 0
		for _, pr := range g.Processes {
			if pr.UID == "" {
				p.UserValid = false
			}
			if pr.UID != "" {
				p.Users[pr.UID] = pr.User
			}
		}
		// 事件指纹排除不断变化的显存和观测时间，只在进程身份、权限或健康状态变化时记录。
		identities := []string{}
		for _, pr := range g.Processes {
			identities = append(identities, fmt.Sprintf("%d/%d/%s/%s/%s", pr.PID, pr.Created, pr.UID, pr.User, pr.Name))
		}
		sort.Strings(identities)
		p.Signature = hash(strings.Join(identities, "\n") + "/" + g.ProcessStatus + "/" + g.Status + "/" + g.Fields["health"])
		// 限定 BootID，避免客户端重启后把两个生命周期的状态连接起来。
		var before, after point
		var raw string
		err := tx.QueryRow(
			"SELECT data FROM points WHERE node=? AND uuid=? AND boot=? AND at<? ORDER BY at DESC LIMIT 1",
			s.NodeID,
			g.UUID,
			s.BootID,
			s.At,
		).Scan(&raw)
		hasBefore := err == nil
		if hasBefore {
			json.Unmarshal([]byte(raw), &before)
		}
		err = tx.QueryRow(
			"SELECT data FROM points WHERE node=? AND uuid=? AND boot=? AND at>? ORDER BY at LIMIT 1",
			s.NodeID,
			g.UUID,
			s.BootID,
			s.At,
		).Scan(&raw)
		hasAfter := err == nil
		if hasAfter {
			json.Unmarshal([]byte(raw), &after)
		}
		if _, e = tx.Exec("INSERT INTO points VALUES(?,?,?,?,?,?)", s.NodeID, g.UUID, s.BootID, s.Seq, s.At, marshal(p)); e != nil {
			return e
		}
		if hasBefore {
			if e = replaceEdge(tx, s.NodeID, g.UUID, s.BootID, before, p); e != nil {
				return e
			}
		}
		if hasAfter {
			if e = replaceEdge(tx, s.NodeID, g.UUID, s.BootID, p, after); e != nil {
				return e
			}
		}
		// 每个采集生命周期只记一次开始；进程变动和恢复正常不写事件。
		if !hasBefore && !hasAfter {
			if _, e = tx.Exec(
				"INSERT INTO events(node,uuid,at,kind,detail) VALUES(?,?,?,?,?)",
				s.NodeID,
				g.UUID,
				s.At,
				"started",
				marshal(map[string]any{"boot": s.BootID}),
			); e != nil {
				return e
			}
		} else if !hasBefore {
			// 补传更早的样本时修正开始时间，不新增一条开始事件。
			if _, e = tx.Exec(
				"UPDATE events SET at=MIN(at,?) WHERE node=? AND uuid=? AND kind='started' AND json_extract(detail,'$.boot')=?",
				s.At,
				s.NodeID,
				g.UUID,
				s.BootID,
			); e != nil {
				return e
			}
		}
		// 连续失败仅记录一次，恢复后再次失败才新增事件。
		if g.Status != "ok" && (!hasBefore || before.Status == "ok") {
			detail := map[string]any{"status": g.Status, "process_status": g.ProcessStatus, "fields": g.Fields}
			if _, e = tx.Exec(
				"INSERT INTO events(node,uuid,at,kind,detail) VALUES(?,?,?,?,?)",
				s.NodeID,
				g.UUID,
				s.At,
				"failure",
				marshal(detail),
			); e != nil {
				return e
			}
		}
		if _, e = tx.Exec(`
			INSERT INTO gpus VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (node, uuid) DO UPDATE SET
				name = excluded.name,
				first_seen = MIN(first_seen, excluded.first_seen),
				last_seen = MAX(last_seen, excluded.last_seen)
		`, s.NodeID, g.UUID, g.Name, s.At, s.At); e != nil {
			return e
		}
	}
	if s.At > oldAt {
		// 只有成功取得清单后缺少已知卡，才记录设备未检测到。
		present := map[string]bool{}
		for _, g := range s.GPUs {
			present[g.UUID] = true
		}
		for _, g := range old.GPUs {
			if !present[g.UUID] && s.GPUStatus == "ok" {
				previous := g.Status
				g.Status = "not_detected"
				g.ProcessStatus = "unknown"
				g.Processes = nil
				s.GPUs = append(s.GPUs, g)
				if previous != "not_detected" {
					if _, e = tx.Exec(
						"INSERT INTO events(node,uuid,at,kind,detail) VALUES(?,?,?,?,?)",
						s.NodeID,
						g.UUID,
						s.At,
						"missing",
						`{"status":"not_detected"}`,
					); e != nil {
						return e
					}
				}
			}
		}
		if _, e = tx.Exec("UPDATE nodes SET snapshot=?,sample_at=? WHERE id=?", marshal(s), s.At, s.NodeID); e != nil {
			return e
		}
	}
	// 接收时间只由足够新鲜的实时上报更新，避免补传积压数据让离线节点“复活”。
	if !s.Backfill && s.At >= oldAt && time.Now().UnixMilli()-s.At <= int64(s.Interval*2000) {
		if _, e = tx.Exec("UPDATE nodes SET received_at=? WHERE id=?", time.Now().UnixMilli(), s.NodeID); e != nil {
			return e
		}
	}
	return tx.Commit()
}
