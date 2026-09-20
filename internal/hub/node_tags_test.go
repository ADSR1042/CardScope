package hub

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNodeTagsAPI(t *testing.T) {
	h := testHub(t)
	node(t, h, "tagged")
	admin, csrf := login(t, h, "admin")
	viewer, viewerCSRF := login(t, h, "viewer")
	settings := nodeSettings{Name: "tagged", Tags: []string{" 训练 ", "机房 A", "训练"}}
	if w := request(h, "PATCH", "/nodes/tagged", settings, viewer, viewerCSRF, ""); w.Code != 403 {
		t.Fatalf("viewer write: %d", w.Code)
	}
	if w := request(h, "PATCH", "/nodes/tagged", settings, admin, csrf, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	check := func(want []string) {
		t.Helper()
		w := request(h, "GET", "/nodes", nil, viewer, "", "")
		var nodes []struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil || len(nodes) != 1 || !reflect.DeepEqual(nodes[0].Tags, want) {
			t.Fatalf("tags response: %s, %v", w.Body.String(), err)
		}
	}
	check([]string{"训练", "机房 A"})
	settings.Tags = nil
	if w := request(h, "PATCH", "/nodes/tagged", settings, admin, csrf, ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	check([]string{"训练", "机房 A"})
	for _, tags := range [][]string{{" "}, {strings.Repeat("中", 33)}, {"bad\nlabel"}, make([]string, 21)} {
		settings.Tags = tags
		if w := request(h, "PATCH", "/nodes/tagged", settings, admin, csrf, ""); w.Code != 400 {
			t.Fatal("invalid tags accepted", w.Code)
		}
	}
	check([]string{"训练", "机房 A"})
	settings.Tags = []string{}
	if w := request(h, "PATCH", "/nodes/tagged", settings, admin, csrf, ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	check([]string{})
}

func TestNodeTagsLegacyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(strings.Replace(schema, "    tags TEXT NOT NULL DEFAULT '[]',\n", "", 1)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO nodes(id,name,created) VALUES('old','old',1)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for i := 0; i < 2; i++ {
		db, err = openStore(path)
		if err != nil {
			t.Fatal(err)
		}
		var tags string
		if err = db.QueryRow("SELECT tags FROM nodes WHERE id='old'").Scan(&tags); err != nil || tags != "[]" {
			t.Fatalf("migration: %s %v", tags, err)
		}
		db.Close()
	}
}
