package hub

import (
	"testing"
	"time"
)

func TestOnlyStartAndFailureEvents(t *testing.T) {
	h := testHub(t)
	node(t, h, "n")
	base := time.Now().Add(-time.Minute).UnixMilli()
	for i := int64(1); i <= 7; i++ {
		s := sample("n", i, base+i*5000)
		if i == 2 {
			s.GPUs[0].Processes = nil
		}
		if i == 3 || i == 4 || i == 6 {
			s.GPUs[0].Status = "query_failed"
			s.GPUs[0].ProcessStatus = "unknown"
		}
		if i == 7 {
			s.BootID = "boot-two"
		}
		if err := ingest(h.DB, s); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := h.DB.Query("SELECT kind,count(*) FROM events GROUP BY kind")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		rows.Scan(&kind, &n)
		counts[kind] = n
	}
	if len(counts) != 2 || counts["started"] != 2 || counts["failure"] != 2 {
		t.Fatalf("unexpected events: %v", counts)
	}
}
