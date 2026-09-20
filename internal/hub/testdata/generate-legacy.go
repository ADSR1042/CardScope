//go:build ignore

// Run from an isolated module pinned to modernc.org/sqlite v1.36.1.
// Arguments: path to store.go, output database path, output gzip path.
package main

import (
	"compress/gzip"
	"database/sql"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	source, err := os.ReadFile(os.Args[1])
	must(err)
	schema := strings.Split(strings.Split(string(source), "const schema = `")[1], "`")[0]
	schema = strings.ReplaceAll(schema, "\r\n", "\n")
	schema = strings.Replace(schema, "    tags TEXT NOT NULL DEFAULT '[]',\n", "", 1)
	schema = strings.Replace(schema, "    attribution_valid REAL NOT NULL DEFAULT 0,\n", "", 1)
	db, err := sql.Open("sqlite", os.Args[2])
	must(err)
	_, err = db.Exec(schema)
	must(err)
	_, err = db.Exec(`
INSERT INTO nodes(id,name,created) VALUES('legacy','Synthetic legacy node',1);
INSERT INTO settings(key,value) VALUES('fixture','sqlite-1.36.1');
INSERT INTO rollups(node,uuid,user_id,res,bucket,occupied) VALUES('legacy','synthetic-gpu','1000',3600000,1,5);
INSERT INTO events(node,uuid,at,kind,detail) VALUES('legacy','synthetic-gpu',1,'test','{}');
`)
	must(err)
	must(db.Close())
	data, err := os.ReadFile(os.Args[2])
	must(err)
	out, err := os.Create(os.Args[3])
	must(err)
	z := gzip.NewWriter(out)
	_, err = z.Write(data)
	must(err)
	must(z.Close())
	must(out.Close())
}
