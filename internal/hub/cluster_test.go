package hub

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestClusterHistoryWeightedAndGaps(t *testing.T) {
	h := testHub(t)
	node(t, h, "cluster-a")
	node(t, h, "cluster-b")
	bucket := time.Now().Add(-time.Hour).UnixMilli() / 900000 * 900000
	// Unequal coverage: (30 + 0) / (60 + 30) = 33.33%, not the 25% mean of means.
	for _, v := range []struct {
		node, gpu, user string
		at              int64
		weighted, valid float64
	}{
		{"cluster-a", "g1", "", bucket, 30, 60},
		{"cluster-b", "g2", "", bucket, 0, 30},
		{"cluster-a", "g1", "user-1", bucket, 60, 60},
		{"deleted-node", "g3", "", bucket, 60, 60},
		{"cluster-a", "g1", "", bucket + 1800000, 0, 60},
	} {
		_, err := h.DB.Exec(upsertRollup, v.node, v.gpu, "test", v.user, "", 60000, v.at, 0, v.weighted, 0, v.valid, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	if w := request(h, "GET", "/cluster/history", nil, "", "", ""); w.Code != 401 {
		t.Fatal("history must require session", w.Code)
	}
	cookie, _ := login(t, h, "viewer")
	w := request(h, "GET", "/cluster/history", nil, cookie, "", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var data struct {
		Points []struct {
			At   int64    `json:"at"`
			Util *float64 `json:"util"`
		} `json:"points"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Points) < 96 || len(data.Points) > 97 {
		t.Fatal("unexpected bucket count", len(data.Points))
	}
	found := 0
	for _, p := range data.Points {
		switch p.At {
		case bucket:
			found++
			if p.Util == nil || math.Abs(*p.Util-100.0/3) > .001 {
				t.Fatal("incorrect weighted average", p.Util)
			}
		case bucket + 900000:
			found++
			if p.Util != nil {
				t.Fatal("missing bucket must be null")
			}
		case bucket + 1800000:
			found++
			if p.Util == nil || *p.Util != 0 {
				t.Fatal("observed idle must be zero")
			}
		}
	}
	if found != 3 {
		t.Fatal("missing expected time buckets")
	}
}
