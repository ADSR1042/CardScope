# 开发与验证

需要 Go 1.26+、Node.js 20.19+（20.x）或 22.12+、npm；Python 辅助脚本需要 Python 3.11+。以下命令从仓库根目录执行，注明例外的除外。

## 本地检查

```sh
cd web
npm ci
npm run format:check
npm run build
node --test tests/dashboard-data.mjs
cd ..
go test ./...
go vet ./...
python -m pip install -r requirements-dev.txt
ruff format --check scripts tests
ruff check scripts tests
```

Go 文件使用 `gofmt`。前端格式化在 `web` 内执行 `npm run format`。

前端开发在 `web` 内运行 `npm run dev`，Vite 将 `/api` 请求代理到 `127.0.0.1:8080`。中心服务可从根目录运行 `go run ./cmd/gpu-hub serve --listen 127.0.0.1:8080 --data .runtime/dev-data`，首次运行会交互设置密码。

## 构建发行包

```sh
sh scripts/build/build.sh
```

Windows 使用 `scripts/build/build.ps1`，从 PATH 查找 Go，也可通过环境变量 `GO_BINARY` 指定 Go 可执行文件路径。两个脚本都先构建前端、运行 Go 测试，再生成 Linux amd64/arm64 发行包到 `dist`。测试报告源文件在 `docs/testing.md`，发行包内仍命名为 `TEST-REPORT.md`。

发行包同时包含普通用户安装脚本，见 [安装与升级](deployment.md)。隔离安装测试使用临时 HOME、模拟 crontab 和无网络的测试进程，不操作真实服务。CI 自动执行，Linux 普通用户也可运行：

```sh
test_dir=$(mktemp -d)
go build -o "$test_dir/service" tests/integration/fixtures/service.go
go build -o "$test_dir/hub" ./cmd/gpu-hub
CARDSCOPE_TEST_BINARY="$test_dir/service" CARDSCOPE_HUB_BINARY="$test_dir/hub" python3 tests/integration/test_install.py -v
```

## 集成测试与浏览器测试

推荐先在 `web` 内运行 `npm run test:smoke`。它自动构建前端和当前平台的中心程序，创建独立临时数据库与本机回环服务，验证登录、节点创建、文案保存、统计页、手机布局和只读权限，最后停止测试进程。无需准备账号文件或真实 GPU，测试产物位于 `.runtime/smoke-*`。需要本机 Edge 和 PATH 中的 Go；Go 不在 PATH 时，将环境变量 `GO_BINARY` 设为 Go 可执行文件的绝对路径。

- `tests/integration/wsl-smoke.py` 在 WSL/Linux 内使用 `/tmp/gpu-monitor-local` 启动测试中心和真实采集客户端，需要先生成 Linux 二进制。
- `tests/integration/recovery-smoke.py` 在上述测试环境中验证独占锁和离线补传。
- `scripts/dev/simulation.py` 生成模拟节点数据；`scripts/dev/demo-processes.py` 是开发辅助负载脚本，执行前阅读其内容。
- 在 `web` 内运行 `npm run test:e2e` 执行主浏览器测试；其他场景使用 `node tests/e2e/e2e-场景名.mjs`。这些测试需要已准备的测试服务、`.runtime` 接入信息以及本机 Edge，并可能修改测试服务数据。
- 浏览器测试通过 `web/tests/e2e/paths.mjs` 定位仓库和 `.runtime`，不依赖启动命令的当前目录。

历史验证记录见 [测试报告](testing.md)，其中的实机结论对应当时的环境，不能替代当前修改后的回归检查。
