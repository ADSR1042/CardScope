# 项目结构

项目在一个仓库内维护 Go 中心服务、Go 采集客户端和 React 前端，使用一个 Go module。前端构建后嵌入中心程序，部署方式保持为单个中心二进制加 SQLite。

```text
cmd/                      可执行程序入口
  gpu-hub/                中心命令行、启动和退出
  gpu-agent/              客户端命令入口
internal/
  agent/                  接入、配置、服务安装、采样循环、队列和上传
  collect/                系统资源、NVML 和 nvidia-smi 采集
  hub/                    HTTP 接口、认证、SQLite 存储和统计
  model/                  客户端与中心共用的数据结构
  webui/                  前端静态资源嵌入
    assets/dist/          Vite 构建产物
web/
  src/
    main.tsx              React 挂载入口
    app/                  应用状态、页面组合和布局
    features/             auth、nodes、gpu、statistics、settings
    components/           跨功能复用的组件
    api/                  统一请求客户端和 CSRF 状态
    lib/                  公共函数
    styles/               全局样式及按顺序加载的样式入口
    types.ts              共用接口类型
  tests/e2e/              浏览器测试
scripts/
  build/                  Windows、Linux 构建和打包
  dev/                    模拟数据与开发辅助
  ops/                    可复用的运维工具
tests/integration/        WSL 实机、离线恢复验证
docs/                     架构、开发、部署、测试说明和公开截图
.runtime/                 本地测试数据、截图、诊断结果，不提交
dist/                     最终发行包，不提交
```

## 代码归属

- `cmd` 负责命令入口及进程启动。客户端实现位于 `internal/agent`，不从入口包导入业务代码。
- `hub/api.go` 保留接口分发、会话鉴权、CSRF 和管理员写操作的锁边界；具体操作放在对应功能文件中。
- 节点管理由 `node_service.go` 负责校验、创建、配置、接入码重签和撤销；`node_mutations.go` 只处理 HTTP 参数与结果映射。业务操作不依赖 HTTP，调用方持有 `Hub.mu`，与上报、接入码兑换共用并发边界。数据库故障返回服务端错误，不当作节点不存在。
- `hub/store.go` 保存数据库初始化和 schema；`ingest.go`、`validation.go`、`rollups.go`、`prune.go` 分别负责入库、校验、积分汇总和清理。这些文件仍属于同一 Go 包。
- 前端组件按业务功能放在 `features`。只被一个功能使用的组件留在功能目录，多个功能共用的组件放入 `components`。
- `features/auth/useSession.ts` 管理会话恢复、登录与退出；`features/nodes/useNodes.ts` 管理节点轮询、刷新和请求状态，忽略旧会话及被后续请求替代的响应。`App.tsx` 负责组合页面和界面交互。GPU 状态判断放在 `features/gpu/status.ts`，`lib/format.ts` 仅保留通用格式化。
- 样式由 `styles/index.css` 统一加载，顺序影响 CSS 层叠。新增或调整样式时注意桌面、手机及深色主题。
- Go 单元测试与被测代码相邻；跨进程测试放在 `tests/integration`，浏览器测试放在 `web/tests/e2e`。
- 客户端 `uploadLoop` 调度 `uploadBatch`，每批最多尝试 20 个队列条目，失败时保留当前样本并停止本批。队列容量与保留时间、失败重试、最新样本优先和取消行为有独立回归测试。

## 构建与临时文件

Vite 将前端写入 `internal/webui/assets/dist`，`webui/embed.go` 将其嵌入中心。`assets/README.txt` 使未构建前端的源码也能进行 Go 检查；发行前必须先构建前端。执行方式见 [开发说明](development.md)。

`.runtime` 中已有的私人维护脚本和连接配置保留原位；它们不属于受版本管理的工具。今后可复用的运维脚本放入 `scripts/ops`，真实凭据通过外部配置传入。不要将 `.runtime` 整体提交或直接提升为公开源码。
