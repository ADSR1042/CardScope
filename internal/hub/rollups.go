package hub

import (
	"database/sql"
	"encoding/json"
)

// point 保留积分所需的最小状态，不重复存储完整进程列表。
// ProcessValid 表示能判断整卡是否被占用；UserValid 还要求所有进程的 UID 可读取。
// 因此用户归属不完整时，仍可记录确定的整卡占用及已知用户的持卡时间。
type point struct {
	At           int64             `json:"at"`
	Interval     int               `json:"interval"`
	Seq          int64             `json:"seq"`
	Model        string            `json:"model"`
	Occupied     bool              `json:"occupied"`
	ProcessValid bool              `json:"process_valid"`
	UserValid    bool              `json:"user_valid"`
	Users        map[string]string `json:"users"`
	Util         *float64          `json:"util"`
	Memory       *float64          `json:"memory"`
	Signature    string            `json:"signature"`
	Status       string            `json:"status"`
}

// contribution 是一个时间桶内的积分增量。时长统一为秒，API 输出时才换算成小时。
// MemorySum 保存“字节 × 秒”，除以 MemoryValid 得到按有效时间加权的平均显存。
// User 为空表示整卡汇总，非空表示该 UID 的持卡汇总，两者不能直接相加。
type contribution struct {
	Node             string
	UUID             string
	Model            string
	User             string
	Name             string
	Res              int64
	Bucket           int64
	Occupied         float64
	Weighted         float64
	OccupancyValid   float64
	UtilValid        float64
	MemorySum        float64
	MemoryValid      float64
	AttributionValid float64
}

// contributions 用前一个样本 a 的状态积分区间 [a.At, b.At)，并按分钟/小时边界切分。
// 超过两倍采样周期的空档视为未知，不延续旧状态；调用方负责只连接同一次启动的样本。
func contributions(node, uuid string, a, b point) []contribution {
	if b.At <= a.At || b.At-a.At > int64(2*a.Interval)*1000 {
		return nil
	}
	out := []contribution{}
	for _, res := range []int64{60000, 3600000} {
		for start := a.At; start < b.At; {
			bucket := start / res * res
			end := min(b.At, bucket+res)
			sec := float64(end-start) / 1000
			r := contribution{Node: node, UUID: uuid, Model: a.Model, Res: res, Bucket: bucket}
			if a.ProcessValid && a.UserValid {
				r.AttributionValid = sec
			}
			if a.ProcessValid {
				r.OccupancyValid = sec
				if a.Occupied {
					r.Occupied = sec
				}
			}
			if a.Util != nil {
				r.UtilValid = sec
				r.Weighted = sec * *a.Util / 100
			}
			if a.Memory != nil {
				r.MemoryValid = sec
				r.MemorySum = sec * *a.Memory
			}
			out = append(out, r)
			if a.ProcessValid {
				// Users 已按 UID 去重。同卡多进程只算一份；多人共享时每个人各算一份。
				for uid, name := range a.Users {
					out = append(out, contribution{
						Node:           node,
						UUID:           uuid,
						Model:          a.Model,
						User:           uid,
						Name:           name,
						Res:            res,
						Bucket:         bucket,
						Occupied:       sec,
						OccupancyValid: sec,
					})
				}
			}
			start = end
		}
	}
	return out
}

// applyContributions 用 sign=1 累加、sign=-1 撤销；两步都在同一事务内完成。
func applyContributions(tx *sql.Tx, cs []contribution, sign float64) error {
	for _, c := range cs {
		_, e := tx.Exec(
			upsertRollup,
			c.Node,
			c.UUID,
			c.Model,
			c.User,
			c.Name,
			c.Res,
			c.Bucket,
			sign*c.Occupied,
			sign*c.Weighted,
			sign*c.OccupancyValid,
			sign*c.UtilValid,
			sign*c.MemorySum,
			sign*c.MemoryValid,
			sign*c.AttributionValid,
		)
		if e != nil {
			return e
		}
	}
	return nil
}

// replaceEdge 以右端样本序号标识一个积分区间，先扣除旧贡献，再写入新贡献。
// 例如 A、C 已入库，补到 B 后分别写入 A→B、B→C；后者会撤销原来的 A→C。
func replaceEdge(tx *sql.Tx, node, uuid, boot string, a, b point) error {
	var old string
	e := tx.QueryRow(
		"SELECT data FROM edges WHERE node=? AND uuid=? AND boot=? AND right_seq=?",
		node,
		uuid,
		boot,
		b.Seq,
	).Scan(&old)
	if e == nil {
		var cs []contribution
		json.Unmarshal([]byte(old), &cs)
		if e = applyContributions(tx, cs, -1); e != nil {
			return e
		}
	} else if e != sql.ErrNoRows {
		return e
	}
	cs := contributions(node, uuid, a, b)
	if e = applyContributions(tx, cs, 1); e != nil {
		return e
	}
	_, e = tx.Exec("INSERT OR REPLACE INTO edges VALUES(?,?,?,?,?)", node, uuid, boot, b.Seq, marshal(cs))
	return e
}
