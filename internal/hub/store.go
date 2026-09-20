package hub

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	_ "modernc.org/sqlite"
)

// 存储分为三层：samples/points 保存原始观测，edges 保存相邻观测的积分贡献，
// rollups 保存分钟/小时汇总。edges 允许补传插入历史样本时撤销旧积分，再计算新积分。
// nodes.snapshot 仅服务实时页面；events 仅保存进程或状态变化，避免重复记录进程表。
const schema = `
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS accounts (
    username TEXT PRIMARY KEY,
    password TEXT NOT NULL,
    role TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
    hash TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    csrf TEXT NOT NULL,
    expires INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL DEFAULT '',
    code_hash TEXT NOT NULL DEFAULT '',
    code_expires INTEGER NOT NULL DEFAULT 0,
    created INTEGER NOT NULL,
    expected INTEGER NOT NULL DEFAULT 0,
    networks TEXT NOT NULL DEFAULT '[]',
    tags TEXT NOT NULL DEFAULT '[]',
    snapshot TEXT NOT NULL DEFAULT '{}',
    sample_at INTEGER NOT NULL DEFAULT 0,
    received_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS samples (
    node TEXT,
    boot TEXT,
    seq INTEGER,
    at INTEGER,
    system TEXT,
    PRIMARY KEY (node, boot, seq)
);
CREATE INDEX IF NOT EXISTS samples_at ON samples(at);
CREATE TABLE IF NOT EXISTS gpus (
    node TEXT,
    uuid TEXT,
    name TEXT,
    first_seen INTEGER,
    last_seen INTEGER,
    PRIMARY KEY (node, uuid)
);
CREATE TABLE IF NOT EXISTS points (
    node TEXT,
    uuid TEXT,
    boot TEXT,
    seq INTEGER,
    at INTEGER,
    data TEXT,
    PRIMARY KEY (node, uuid, boot, seq)
);
CREATE INDEX IF NOT EXISTS points_time ON points(node,uuid,boot,at);
CREATE TABLE IF NOT EXISTS edges (
    node TEXT,
    uuid TEXT,
    boot TEXT,
    right_seq INTEGER,
    data TEXT,
    PRIMARY KEY (node, uuid, boot, right_seq)
);
CREATE TABLE IF NOT EXISTS rollups (
    node TEXT,
    uuid TEXT,
    model TEXT,
    user_id TEXT,
    user_name TEXT,
    res INTEGER,
    bucket INTEGER,
    occupied REAL,
    weighted REAL,
    occupancy_valid REAL,
    util_valid REAL,
    memory_sum REAL,
    memory_valid REAL,
    attribution_valid REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (node, uuid, user_id, res, bucket)
);
CREATE INDEX IF NOT EXISTS rollups_time ON rollups(res,bucket);
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node TEXT,
    uuid TEXT,
    at INTEGER,
    kind TEXT,
    detail TEXT
);
CREATE INDEX IF NOT EXISTS events_time ON events(node,uuid,at);
`

const upsertRollup = `
INSERT INTO rollups VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (node, uuid, user_id, res, bucket) DO UPDATE SET
    occupied = occupied + excluded.occupied,
    weighted = weighted + excluded.weighted,
    occupancy_valid = occupancy_valid + excluded.occupancy_valid,
    util_valid = util_valid + excluded.util_valid,
    memory_sum = memory_sum + excluded.memory_sum,
    memory_valid = memory_valid + excluded.memory_valid,
    attribution_valid = attribution_valid + excluded.attribution_valid,
    user_name = excluded.user_name
`

func openStore(path string) (*sql.DB, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	// 小规模部署使用单连接简化 SQLite 写入竞争。事务内必须使用 tx，
	// 不能再通过 db 申请连接，否则会等待自己占用的唯一连接。
	db.SetMaxOpenConns(1)
	statements := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		schema,
	}
	for _, s := range statements {
		if _, e = db.Exec(s); e != nil {
			db.Close()
			return nil, e
		}
	}
	rows, e := db.Query("PRAGMA table_info(rollups)")
	if e != nil {
		db.Close()
		return nil, e
	}
	hasAttribution := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var def any
		rows.Scan(&cid, &name, &kind, &notnull, &def, &pk)
		if name == "attribution_valid" {
			hasAttribution = true
		}
	}
	rows.Close()
	if !hasAttribution {
		// 旧版本没有用户归属覆盖率；新增列从零开始，不能把旧数据推定为归属完整。
		if _, e = db.Exec("ALTER TABLE rollups ADD COLUMN attribution_valid REAL NOT NULL DEFAULT 0"); e != nil {
			db.Close()
			return nil, e
		}
	}
	var hasTags int
	if e = db.QueryRow("SELECT count(*) FROM pragma_table_info('nodes') WHERE name='tags'").Scan(&hasTags); e == nil && hasTags == 0 {
		_, e = db.Exec("ALTER TABLE nodes ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'")
	}
	if e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}

// hash 用于高熵随机令牌和事件指纹；登录密码另用 bcrypt，不能用此函数代替。
func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func marshal(v any) string { b, _ := json.Marshal(v); return string(b) }
