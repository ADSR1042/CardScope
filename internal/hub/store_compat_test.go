package hub

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLite136FileMigrationAndReopen(t *testing.T) {
	fixture, err := os.Open("testdata/sqlite-1.36.1.db.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	z, err := gzip.NewReader(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	data, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.db")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		db, err := openStore(path)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var tags, name, marker, integrity string
		if err := db.QueryRow("SELECT name,tags FROM nodes WHERE id='legacy'").Scan(&name, &tags); err != nil || name != "Synthetic legacy node" || tags != "[]" {
			t.Fatalf("legacy node migration: %q %q %v", name, tags, err)
		}
		if err := db.QueryRow("SELECT value FROM settings WHERE key='fixture'").Scan(&marker); err != nil || marker != "sqlite-1.36.1" {
			t.Fatalf("legacy setting: %q %v", marker, err)
		}
		var occupied, attribution float64
		if err := db.QueryRow("SELECT occupied,attribution_valid FROM rollups WHERE node='legacy'").Scan(&occupied, &attribution); err != nil || occupied != 5 || attribution != 0 {
			t.Fatalf("legacy rollup: %v %v %v", occupied, attribution, err)
		}
		if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
			t.Fatalf("integrity: %s %v", integrity, err)
		}
		if i == 1 {
			if err := prune(db); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"events", "rollups"} {
				var count int
				if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
					t.Fatalf("legacy retention %s: %d %v", table, count, err)
				}
			}
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
