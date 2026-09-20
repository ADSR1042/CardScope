#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/../.."
(cd web && npm ci && npm run build)
go test ./...
for arch in amd64 arm64; do
  mkdir -p "dist/linux-$arch"
  for app in gpu-agent gpu-hub; do
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -buildvcs=false -trimpath -ldflags='-s -w' -o "dist/linux-$arch/$app" "./cmd/$app"
  done
  cp README.md "dist/linux-$arch/README.md"
  cp LICENSE "dist/linux-$arch/LICENSE"
  cp docs/testing.md "dist/linux-$arch/TEST-REPORT.md"
  tar -czf "dist/gpu-monitor-linux-$arch.tar.gz" -C dist "linux-$arch"
done
(cd dist && sha256sum ./*.tar.gz > SHA256SUMS)
