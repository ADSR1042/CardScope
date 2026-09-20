`sqlite-1.36.1.db.gz` is a synthetic database written by modernc.org/sqlite
v1.36.1. It contains no credentials or deployment data. Its schema predates
the node tags and rollup attribution columns. The compatibility test opens
it with the current driver, verifies both migrations and preserved records,
reopens it, checks database integrity and exercises retention cleanup.

To regenerate, build `generate-legacy.go` in an isolated Go module pinned to
modernc.org/sqlite v1.36.1 (the go.mod and go.sum from commit b27ea59 can be
used). Run the resulting executable with three arguments: the absolute path
to `internal/hub/store.go`, a new temporary database path, and the absolute
output path `internal/hub/testdata/sqlite-1.36.1.db.gz`.
