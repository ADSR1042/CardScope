# 参与贡献

感谢你愿意改进 CardScope。提交改动前，请先确认问题可以稳定复现，并尽量让每个提交只处理一件事。

## 开发环境

修改前请阅读 [项目结构](docs/architecture.md) 和 [开发说明](docs/development.md)，按功能归属放置源码、测试和辅助脚本。

- Go 1.26 或更高版本
- Node.js 20.19+（20.x）或 22.12+
- npm
- Python 3.11 或更高版本，仅用于运行辅助验证脚本

安装依赖并运行检查：

```bash
python -m pip install -r requirements-dev.txt

cd web
npm ci
npm run format:check
npm run build
cd ..

go test ./...
ruff format --check scripts tests
ruff check scripts tests
```

修改前端后可运行 `npm run format`。Go 文件使用 `gofmt`，Python 文件使用 Ruff，行宽为 100。

## 提交问题

问题报告应包含操作系统、GPU 与驱动版本、程序版本、复现步骤、预期结果和实际结果。请先移除日志中的账号、节点凭据、接入码、内网地址、主机名和 GPU UUID。

安全问题不要提交公开 Issue，请私下联系维护者。

## 提交合并请求

1. 从独立分支提交改动。
2. 为行为变化补充或更新测试。
3. 确认 Go、前端和 Python 检查通过。
4. 在说明中写清动机、实现方式、验证结果和兼容性影响。
5. 不要提交 `.runtime`、数据库、日志、真实凭据或生产环境截图。

项目默认使用中文文档和界面文字；代码标识符保持英文。

除非贡献者另有明确声明，提交给本项目的贡献按照 [Apache License 2.0](LICENSE) 授权。
