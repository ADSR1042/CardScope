package hub

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNodeServiceCredentialLifecycle(t *testing.T) {
	h, err := Open(filepath.Join(t.TempDir(), "nodes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	service := nodeService{h.DB}
	h.mu.Lock()
	defer h.mu.Unlock()
	node, err := service.create("test-node")
	if err != nil {
		t.Fatal(err)
	}
	if err = service.requireNode(node.NodeID); err != nil {
		t.Fatal(err)
	}
	var storedCode, storedToken string
	var expires int64
	check := func(code string) {
		t.Helper()
		if err := h.DB.QueryRow("SELECT code_hash,token_hash,code_expires FROM nodes WHERE id=?", node.NodeID).Scan(&storedCode, &storedToken, &expires); err != nil {
			t.Fatal(err)
		}
		if storedCode != hash(code) || expires <= time.Now().UnixMilli() {
			t.Fatal("enrollment code is not stored as an expiring hash")
		}
	}
	check(node.Code)
	if _, err = h.DB.Exec("UPDATE nodes SET token_hash=? WHERE id=?", hash("previous-token"), node.NodeID); err != nil {
		t.Fatal(err)
	}
	code, err := service.issueCode(node.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	check(code)
	if code == node.Code || storedToken != "" {
		t.Fatal("reissuing enrollment failed to invalidate previous credentials")
	}
	if err = service.revoke(node.NodeID); err != nil {
		t.Fatal(err)
	}
	if err = h.DB.QueryRow("SELECT code_hash,token_hash,code_expires FROM nodes WHERE id=?", node.NodeID).Scan(&storedCode, &storedToken, &expires); err != nil {
		t.Fatal(err)
	}
	if storedCode != "" || storedToken != "" || expires != 0 {
		t.Fatal("revocation left active credentials")
	}
}

func TestNodeServiceValidationAndMissingNodes(t *testing.T) {
	h, err := Open(filepath.Join(t.TempDir(), "nodes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	service := nodeService{h.DB}
	if _, err = service.create("   "); err == nil {
		t.Fatal("blank node name accepted")
	}
	node, err := service.create("original")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []nodeSettings{
		{Name: "changed", Expected: -1},
		{Name: "changed", Expected: 129},
		{Name: "changed", Networks: []string{strings.Repeat("n", 129)}},
	} {
		var validation nodeInputError
		if err = service.configure(node.NodeID, input); !errors.As(err, &validation) {
			t.Fatalf("invalid settings returned %v", err)
		}
	}
	var name string
	if err = h.DB.QueryRow("SELECT name FROM nodes WHERE id=?", node.NodeID).Scan(&name); err != nil || name != "original" {
		t.Fatalf("invalid settings changed node: %s, %v", name, err)
	}
	if err = service.configure(node.NodeID, nodeSettings{Name: "changed", Expected: 4, Networks: []string{"eth0"}}); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		service.requireNode("missing"), service.revoke("missing"),
		service.configure("missing", nodeSettings{Name: "valid"}),
	} {
		if !errors.Is(err, errNodeNotFound) {
			t.Fatalf("missing node returned %v", err)
		}
	}
	if _, err = service.issueCode("missing"); !errors.Is(err, errNodeNotFound) {
		t.Fatalf("missing enrollment returned %v", err)
	}
	// A failed database is an internal error, not evidence that a node is absent.
	if err = h.DB.Close(); err != nil {
		t.Fatal(err)
	}
	err = service.requireNode(node.NodeID)
	w := httptest.NewRecorder()
	nodeFailure(w, err, "节点查询失败")
	if w.Code != 500 {
		t.Fatalf("database failure returned HTTP %d", w.Code)
	}
}
