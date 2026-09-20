$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Push-Location $projectRoot
try {
    $goName = if ($env:GO_BINARY) { $env:GO_BINARY } else { 'go' }
    $goCommand = Get-Command $goName -CommandType Application -ErrorAction SilentlyContinue
    if (-not $goCommand) { throw 'Install Go 1.26+ on PATH or set GO_BINARY to its executable path.' }
    $goExe = $goCommand.Source
    Push-Location web
    try { npm.cmd ci; if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }; npm.cmd run build; if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' } } finally { Pop-Location }
    & $goExe test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    foreach ($arch in @('amd64', 'arm64')) {
        $env:GOARCH = $arch
        $out = Join-Path $projectRoot "dist/linux-$arch"
        New-Item -ItemType Directory -Force $out | Out-Null
        foreach ($app in @('gpu-agent', 'gpu-hub')) {
            & $goExe build -buildvcs=false -trimpath -ldflags='-s -w' -o "$out/$app" "./cmd/$app"
            if ($LASTEXITCODE -ne 0) { throw "$app build failed" }
        }
        Copy-Item README.md "$out/README.md"
        Copy-Item LICENSE "$out/LICENSE"
        Copy-Item docs/testing.md "$out/TEST-REPORT.md"
        Copy-Item scripts/ops/install-hub.sh,scripts/ops/install-agent.sh,scripts/ops/install-user.sh,scripts/ops/ensure-running.py $out
        tar.exe -czf "dist/gpu-monitor-linux-$arch.tar.gz" -C dist "linux-$arch"
        if ($LASTEXITCODE -ne 0) { throw 'Archive failed' }
    }
    Get-FileHash dist/*.tar.gz -Algorithm SHA256 | ForEach-Object { "$($_.Hash.ToLower())  $(Split-Path -Leaf $_.Path)" } | Set-Content dist/SHA256SUMS -Encoding ascii
} finally { Pop-Location }
