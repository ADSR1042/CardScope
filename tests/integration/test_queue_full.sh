#!/usr/bin/env bash
# Run a compiled agent test binary in an isolated mount namespace.
# Usage: sudo unshare -m bash tests/integration/test_queue_full.sh /absolute/agent.test
set -euo pipefail
binary=$(realpath "${1:?provide the compiled agent test binary}")
target=$(mktemp -d /tmp/cardscope-ring-full.XXXXXX)
cleanup() {
  if mountpoint -q "$target"; then umount "$target"; fi
  rmdir "$target"
}
trap cleanup EXIT
mount -t tmpfs -o size=16m tmpfs "$target"
CARDSCOPE_FULL_DISK_DIR="$target" "$binary" -test.v -test.run '^TestQueueOnFullFilesystem$'
