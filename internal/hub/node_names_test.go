package hub

import (
	"strings"
	"sync"
	"testing"
)

func TestNodeNamesUnique(t *testing.T) {
	h := testHub(t)
	cookie, csrf := login(t, h, "admin")
	create := func(name string) int {
		return request(h, "POST", "/nodes", map[string]string{"name": name}, cookie, csrf, "").Code
	}
	if code := create("  Worker-A  "); code != 201 {
		t.Fatal(code)
	}
	for _, name := range []string{"Worker-A", "worker-a", " Worker-A "} {
		if code := create(name); code != 400 {
			t.Fatalf("duplicate %q: %d", name, code)
		}
	}
	node(t, h, "second")
	w := request(h, "PATCH", "/nodes/second", nodeSettings{Name: "WORKER-A", Tags: []string{"changed"}}, cookie, csrf, "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "节点名称已存在") {
		t.Fatal(w.Code, w.Body.String())
	}
	var name, tags string
	if err := h.DB.QueryRow("SELECT name,tags FROM nodes WHERE id='second'").Scan(&name, &tags); err != nil || name != "second" || tags != "[]" {
		t.Fatalf("failed rename changed data: %s %s %v", name, tags, err)
	}
	if w := request(h, "PATCH", "/nodes/second", nodeSettings{Name: " second "}, cookie, csrf, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(h, "PATCH", "/nodes/second", nodeSettings{Name: "   "}, cookie, csrf, ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- create("concurrent") }()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[400] != 1 {
		t.Fatal(counts)
	}
}
