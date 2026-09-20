package hub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gpu-monitor/internal/model"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testHub(t *testing.T) *Hub {
	t.Helper()
	h, e := Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(h.Close)
	if e = h.Initialize("admin-test-password", "viewer-test-password"); e != nil {
		t.Fatal(e)
	}
	return h
}

func request(h *Hub, method, path string, body any, cookie, csrf, bearer string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://example.test/api/v1"+path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		r.Header.Set("Cookie", cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, r)
	return w
}

func login(t *testing.T, h *Hub, name string) (string, string) {
	t.Helper()
	w := request(
		h,
		"POST",
		"/login",
		map[string]string{"username": name, "password": name + "-test-password"},
		"",
		"",
		"",
	)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var u user
	json.Unmarshal(w.Body.Bytes(), &u)
	return w.Result().Cookies()[0].Name + "=" + w.Result().Cookies()[0].Value, u.CSRF
}

func node(t *testing.T, h *Hub, id string) {
	t.Helper()
	_, e := h.DB.Exec(
		"INSERT INTO nodes(id,name,created,token_hash) VALUES(?,?,?,?)",
		id,
		id,
		time.Now().UnixMilli(),
		hash("credential"),
	)
	if e != nil {
		t.Fatal(e)
	}
}

func sample(id string, seq, at int64) model.Snapshot {
	return model.Snapshot{
		Version:   1,
		NodeID:    id,
		BootID:    "boot-one",
		Seq:       seq,
		At:        at,
		Interval:  5,
		Hostname:  "test",
		GPUStatus: "ok",
		System: model.System{
			Cores:           8,
			MemoryTotal:     32 << 30,
			MemoryAvailable: 16 << 30,
		},
		GPUs: []model.GPU{
			{
				UUID:          "GPU-test",
				Name:          "NVIDIA Test",
				Status:        "ok",
				Util:          model.Number(50),
				MemoryTotal:   model.Number(24 << 30),
				MemoryUsed:    model.Number(8 << 30),
				ProcessStatus: "ok",
				Fields:        map[string]string{},
				Processes: []model.Process{
					{PID: 12, UID: "1000", User: "alice", Created: 100},
					{PID: 13, UID: "1000", User: "alice", Created: 101},
					{PID: 14, UID: "1001", User: "bob", Created: 102},
				},
			},
		},
	}
}

func TestAuthenticationEnrollmentAndRevocation(t *testing.T) {
	h := testHub(t)
	admin, csrf := login(t, h, "admin")
	viewer, vcsrf := login(t, h, "viewer")
	if w := request(h, "GET", "/nodes", nil, "", "", ""); w.Code != 401 {
		t.Fatal("anonymous access")
	}
	if w := request(h, "POST", "/nodes", map[string]string{"name": "n1"}, viewer, vcsrf, ""); w.Code != 403 {
		t.Fatal("viewer mutation")
	}
	if w := request(h, "POST", "/nodes", map[string]string{"name": "n1"}, admin, "", ""); w.Code != 403 {
		t.Fatal("missing csrf")
	}
	w := request(h, "POST", "/nodes", map[string]string{"name": "n1"}, admin, csrf, "")
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var d map[string]string
	json.Unmarshal(w.Body.Bytes(), &d)
	code := d["code"]
	w = request(h, "POST", "/enroll", map[string]string{"code": code}, "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &d)
	credential := d["token"]
	id := d["node_id"]
	if request(h, "POST", "/enroll", map[string]string{"code": code}, "", "", "").Code != 401 {
		t.Fatal("code reuse")
	}
	s := sample(id, 1, time.Now().Add(-time.Second).UnixMilli())
	if w = request(h, "POST", "/snapshots", s, "", "", credential); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.NodeID = "another"
	if request(h, "POST", "/snapshots", s, "", "", credential).Code != 400 {
		t.Fatal("cross-node upload")
	}
	if request(h, "GET", "/nodes", nil, "", "", credential).Code != 401 {
		t.Fatal("agent read privilege")
	}
	if request(h, "POST", "/nodes/"+id+"/revoke", map[string]string{}, admin, csrf, "").Code != 200 {
		t.Fatal("revoke")
	}
	s.NodeID = id
	if request(h, "POST", "/snapshots", s, "", "", credential).Code != 401 {
		t.Fatal("revoked credential accepted")
	}
}
func TestEnrollmentConcurrentAndExpiry(t *testing.T) {
	h := testHub(t)
	code := token()
	h.DB.Exec(
		"INSERT INTO nodes(id,name,code_hash,code_expires,created) VALUES('n','n',?,?,?)",
		hash(code),
		time.Now().Add(time.Minute).UnixMilli(),
		time.Now().UnixMilli(),
	)
	var wg sync.WaitGroup
	ch := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- request(h, "POST", "/enroll", map[string]string{"code": code}, "", "", "").Code
		}()
	}
	wg.Wait()
	close(ch)
	success := 0
	for c := range ch {
		if c == 200 {
			success++
		}
	}
	if success != 1 {
		t.Fatal("enrollment must be single-use")
	}
	code = token()
	h.DB.Exec("UPDATE nodes SET code_hash=?,code_expires=?", hash(code), time.Now().Add(-time.Second).UnixMilli())
	if request(h, "POST", "/enroll", map[string]string{"code": code}, "", "", "").Code != 401 {
		t.Fatal("expired code")
	}
}
func TestHoursReplaySharingAndMissing(t *testing.T) {
	h := testHub(t)
	node(t, h, "n")
	base := time.Now().Add(-time.Minute).Truncate(time.Minute).UnixMilli() + 1000
	for _, seq := range []int64{1, 3, 2, 2} {
		if e := ingest(h.DB, sample("n", seq, base+(seq-1)*5000)); e != nil {
			t.Fatal(e)
		}
	}
	var occupied, weighted, valid float64
	h.DB.QueryRow(`
		SELECT SUM(occupied), SUM(weighted), SUM(occupancy_valid)
		FROM rollups
		WHERE user_id = '' AND res = 60000
	`).Scan(&occupied, &weighted, &valid)
	if occupied != 10 || weighted != 5 || valid != 10 {
		t.Fatalf("wrong totals %v %v %v", occupied, weighted, valid)
	}
	var userTotal float64
	h.DB.QueryRow("SELECT SUM(occupied) FROM rollups WHERE user_id!='' AND res=60000").Scan(&userTotal)
	if userTotal != 20 {
		t.Fatalf("shared users count, got %v", userTotal)
	}
	gap := sample("n", 4, base+40000)
	if e := ingest(h.DB, gap); e != nil {
		t.Fatal(e)
	}
	h.DB.QueryRow("SELECT SUM(occupied) FROM rollups WHERE user_id='' AND res=60000").Scan(&occupied)
	if occupied != 10 {
		t.Fatal("gap filled")
	}
	restart := sample("n", 1, base+45000)
	restart.BootID = "boot-two"
	ingest(h.DB, restart)
	h.DB.QueryRow("SELECT SUM(occupied) FROM rollups WHERE user_id='' AND res=60000").Scan(&occupied)
	if occupied != 10 {
		t.Fatal("cross boot integration")
	}
	missing := sample("n", 2, base+50000)
	missing.BootID = "boot-two"
	missing.GPUs = nil
	ingest(h.DB, missing)
	var raw string
	h.DB.QueryRow("SELECT snapshot FROM nodes WHERE id='n'").Scan(&raw)
	var state model.Snapshot
	json.Unmarshal([]byte(raw), &state)
	if len(state.GPUs) != 1 || state.GPUs[0].Status != "not_detected" {
		t.Fatal("missing GPU disappeared")
	}
}
func TestNullMetricsClockAndBackfill(t *testing.T) {
	h := testHub(t)
	node(t, h, "n")
	base := time.Now().Add(-time.Hour).UnixMilli()
	s := sample("n", 1, base)
	s.Backfill = true
	s.GPUs[0].ProcessStatus = "permission_denied"
	s.GPUs[0].Util = nil
	ingest(h.DB, s)
	s.Seq++
	s.At += 5000
	ingest(h.DB, s)
	var valid float64
	h.DB.QueryRow("SELECT COALESCE(SUM(occupancy_valid+util_valid),0) FROM rollups").Scan(&valid)
	if valid != 0 {
		t.Fatal("null counted as observation")
	}
	var received int64
	h.DB.QueryRow("SELECT received_at FROM nodes").Scan(&received)
	if received != 0 {
		t.Fatal("backfill marks online")
	}
	future := sample("n", 9, time.Now().Add(time.Hour).UnixMilli())
	if validate(future) == nil {
		t.Fatal("future accepted")
	}
	future = sample("n", 9, time.Now().UnixMilli())
	future.GPUs[0].Util = model.Number(110)
	if validate(future) == nil {
		t.Fatal("invalid utilization")
	}
}
func TestSecurityHeadersRateAndCSV(t *testing.T) {
	h := testHub(t)
	for _, s := range []string{"=cmd", " +SUM(1)", "@x", "-12", "\tstuff"} {
		if !strings.HasPrefix(safeCSV(s), "'") {
			t.Fatalf("CSV unsafe %q", s)
		}
	}
	if safeCSV("alice") != "alice" {
		t.Fatal("ordinary CSV changed")
	}
	for i := 0; i < 11; i++ {
		w := request(h, "POST", "/login", map[string]string{"username": "bad", "password": "bad"}, "", "", "")
		if i == 10 && w.Code != 429 {
			t.Fatal("no rate limit")
		}
	}
	r := httptest.NewRequest("POST", "http://example.test/api/v1/login", strings.NewReader(`{}`))
	r.Header.Set("Origin", "http://evil.test")
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin")
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}
func TestSimulated20Nodes8GPUsAndRetention(t *testing.T) {
	h := testHub(t)
	base := time.Now().Add(-time.Minute).UnixMilli()
	start := time.Now()
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("sim-%02d", i)
		node(t, h, id)
		for seq := int64(1); seq <= 3; seq++ {
			s := sample(id, seq, base+seq*5000)
			g := s.GPUs[0]
			s.GPUs = nil
			for j := 0; j < 8; j++ {
				x := g
				x.UUID = fmt.Sprintf("GPU-%02d-%02d", i, j)
				x.Index = j
				if j == 7 && seq == 2 {
					x.Status = "timeout"
					x.ProcessStatus = "unknown"
				}
				s.GPUs = append(s.GPUs, x)
			}
			if e := ingest(h.DB, s); e != nil {
				t.Fatal(e)
			}
		}
	}
	var count int
	h.DB.QueryRow("SELECT count(*) FROM gpus").Scan(&count)
	if count != 160 {
		t.Fatal(count)
	}
	t.Logf("20 nodes / 160 GPUs / 480 observations ingested in %s", time.Since(start))
	h.DB.Exec("INSERT INTO events(node,uuid,at,kind,detail) VALUES('old','old',0,'test','{}')")
	if e := prune(h.DB); e != nil {
		t.Fatal(e)
	}
	h.DB.QueryRow("SELECT count(*) FROM events WHERE node='old'").Scan(&count)
	if count != 0 {
		t.Fatal("retention not applied")
	}
}
func TestMinuteBoundarySplit(t *testing.T) {
	a := point{At: 59000, Interval: 5, Occupied: true, ProcessValid: true, Util: model.Number(50)}
	b := point{At: 64000}
	cs := contributions("n", "g", a, b)
	var sum float64
	count := 0
	for _, c := range cs {
		if c.Res == 60000 {
			sum += c.Occupied
			count++
		}
	}
	if sum != 5 || count != 2 {
		t.Fatal(cs)
	}
}
func TestPasswordRevokesSessions(t *testing.T) {
	h := testHub(t)
	a, c := login(t, h, "admin")
	v, _ := login(t, h, "viewer")
	w := request(h, "POST", "/accounts/password", map[string]string{"username": "viewer", "password": "new-viewer-password"}, a, c, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if request(h, "GET", "/nodes", nil, v, "", "").Code != http.StatusUnauthorized {
		t.Fatal("old session valid")
	}
}

func TestUnknownUserKeepsCardHoursButReducesAttribution(t *testing.T) {
	h := testHub(t)
	node(t, h, "n")
	base := time.Now().Add(-time.Minute).UnixMilli()
	a := sample("n", 1, base)
	a.GPUs[0].Processes[2].UID = ""
	a.GPUs[0].Processes[2].User = "未知"
	b := sample("n", 2, base+5000)
	if e := ingest(h.DB, a); e != nil {
		t.Fatal(e)
	}
	if e := ingest(h.DB, b); e != nil {
		t.Fatal(e)
	}
	var occupied, attribution float64
	h.DB.QueryRow("SELECT SUM(occupied),SUM(attribution_valid) FROM rollups WHERE res=60000 AND user_id=''").Scan(&occupied, &attribution)
	if occupied != 5 || attribution != 0 {
		t.Fatalf("card occupancy %v attribution %v", occupied, attribution)
	}
}

func TestHistoryEmitsNullForMissingBuckets(t *testing.T) {
	h := testHub(t)
	node(t, h, "n")
	base := time.Now().Add(-5 * time.Minute).Truncate(time.Minute).UnixMilli()
	for i, offset := range []int64{1000, 6000, 181000, 186000} {
		if e := ingest(h.DB, sample("n", int64(i+1), base+offset)); e != nil {
			t.Fatal(e)
		}
	}
	cookie, _ := login(t, h, "viewer")
	w := request(h, "GET", fmt.Sprintf("/history?node=n&gpu=GPU-test&from=%d&to=%d", base, base+240000), nil, cookie, "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result struct {
		Points []struct {
			Util *float64 `json:"util"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Points) != 4 || result.Points[1].Util != nil || result.Points[2].Util != nil {
		t.Fatal("trend bridged missing interval", w.Body.String())
	}
}
