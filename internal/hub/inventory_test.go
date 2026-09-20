package hub

import (
	"encoding/json"
	"gpu-monitor/internal/model"
	"testing"
	"time"
)

func TestInventoryFailureDoesNotMeanMissing(t *testing.T) {
	h := testHub(t)
	node(t, h, "n")
	base := time.Now().Add(-time.Minute).UnixMilli()
	put := func(s model.Snapshot) {
		t.Helper()
		if e := ingest(h.DB, s); e != nil {
			t.Fatal(e)
		}
	}
	put(sample("n", 1, base))
	failure := sample("n", 2, base+5000)
	failure.GPUStatus = "inventory_failed"
	failure.GPUs = nil
	put(failure)
	var raw string
	h.DB.QueryRow("SELECT snapshot FROM nodes WHERE id='n'").Scan(&raw)
	var state model.Snapshot
	json.Unmarshal([]byte(raw), &state)
	if len(state.GPUs) != 1 || state.GPUs[0].Status != "query_failed" || state.GPUs[0].Util != nil || state.GPUs[0].Power != nil {
		t.Fatal("stale or missing GPU", raw)
	}
	var count int
	h.DB.QueryRow("SELECT count(*) FROM events WHERE kind='missing'").Scan(&count)
	if count != 0 {
		t.Fatal("inventory failure created missing event")
	}
	put(sample("n", 3, base+10000))
	var valid float64
	h.DB.QueryRow("SELECT SUM(occupancy_valid) FROM rollups WHERE user_id='' AND res=60000").Scan(&valid)
	if valid != 5 {
		t.Fatalf("failed interval counted as valid: %v", valid)
	}
	// 真正成功枚举但清单为空，仍然保留未检测到事件。
	missing := sample("n", 4, base+15000)
	missing.GPUs = nil
	put(missing)
	h.DB.QueryRow("SELECT count(*) FROM events WHERE kind='missing'").Scan(&count)
	if count != 1 {
		t.Fatal("confirmed missing GPU not recorded")
	}
}
