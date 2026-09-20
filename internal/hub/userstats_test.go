package hub

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGroupUsersByNodeUID(t *testing.T) {
	rows := []statRow{
		{Node: "n1", UUID: "a", UserID: "1", User: "alice", Occupied: 2, OccupancyValid: 1800, Expected: 3600},
		{Node: "n1", UUID: "b", UserID: "1", User: "alice", Occupied: 3, OccupancyValid: 3600, Expected: 3600},
		{Node: "n2", UUID: "c", UserID: "1", User: "alice", Occupied: 1, OccupancyValid: 3600, Expected: 3600},
		{Node: "n1", UUID: "a", Occupied: 50},
	}
	got := groupUserStats(rows)
	if len(got) != 2 || got[0].Hours != 5 || got[0].Cards != 2 || got[0].Coverage != .75 || len(got[0].Details) != 2 || got[1].Node != "n2" {
		t.Fatalf("unexpected groups: %+v", got)
	}
}

func TestUserStatsViewAndExport(t *testing.T) {
	h := testHub(t)
	node(t, h, "n1")
	at := time.Now().Add(-time.Minute).Truncate(time.Minute).UnixMilli()
	for i := int64(0); i < 3; i++ {
		if err := ingest(h.DB, sample("n1", i+1, at+i*5000)); err != nil {
			t.Fatal(err)
		}
	}
	cookie, _ := login(t, h, "viewer")
	w := request(h, "GET", "/stats?view=users&user=alice", nil, cookie, "", "")
	var result struct {
		Users []userStat `json:"users"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 1 || result.Users[0].User != "alice" || result.Users[0].Cards != 1 {
		t.Fatal(w.Body.String())
	}
	// 两个同用户进程不能重复累计；三个样本仅覆盖两个五秒区间。
	if result.Users[0].Hours != 10.0/3600 {
		t.Fatal(result.Users[0].Hours)
	}
	csv := request(h, "GET", "/export?view=users&user=alice", nil, cookie, "", "").Body.String()
	missingAlice := !strings.Contains(csv, "alice")
	unexpectedBob := strings.Contains(csv, "bob")
	unexpectedLegacyColumn := strings.Contains(csv, "weighted_hours")
	missingCardCount := !strings.Contains(csv, "使用卡数")
	if missingAlice || unexpectedBob || unexpectedLegacyColumn || missingCardCount {
		t.Fatal(csv)
	}
}
