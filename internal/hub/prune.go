package hub

import (
	"database/sql"
	"time"
)

// prune 按数据精度分层清理：原始观测 48 小时，分钟汇总和事件 30 天，小时汇总 500 天。
// 清理原始积分边时保留已累计的汇总值，使长期统计不依赖原始样本继续存在。

func prune(db *sql.DB) error {
	now := time.Now().UnixMilli()
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	queries := []struct {
		q string
		n int64
	}{
		{
			q: `
				DELETE FROM edges
				WHERE NOT EXISTS (
					SELECT 1
					FROM points AS p
					WHERE p.node = edges.node
					  AND p.uuid = edges.uuid
					  AND p.boot = edges.boot
					  AND p.seq = edges.right_seq
					  AND p.at >= ?
				)
			`,
			n: now - 48*3600000,
		},
		{q: "DELETE FROM points WHERE at<?", n: now - 48*3600000},
		{q: "DELETE FROM samples WHERE at<?", n: now - 48*3600000},
		{q: "DELETE FROM events WHERE at<?", n: now - 30*86400000},
		{q: "DELETE FROM rollups WHERE res=60000 AND bucket<?", n: now - 30*86400000},
		{q: "DELETE FROM rollups WHERE res=3600000 AND bucket<?", n: now - 500*86400000},
		{q: "DELETE FROM sessions WHERE expires<?", n: now},
	}
	for _, q := range queries {
		if _, e = tx.Exec(q.q, q.n); e != nil {
			return e
		}
	}
	return tx.Commit()
}
