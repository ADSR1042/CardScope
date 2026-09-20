# 内网部署参考

本文给出不依赖 sudo 的通用内网部署方式。示例地址、用户名和路径均为占位符，请按实际环境替换；不要把真实凭据或网络拓扑提交到仓库。

## 部署结构

| 项目 | 中心节点 | GPU 节点 |
| --- | --- | --- |
| 程序 | `gpu-hub` | `gpu-agent` |
| 建议目录 | `~/.local/share/gpu-monitor/` | `~/.local/share/gpu-monitor/` |
| 状态目录 | `~/.local/state/gpu-monitor/` | `~/.config/gpu-monitor/` |
| 入站端口 | TCP 8080 | 无 |

中心节点不需要 GPU。GPU 节点只需能够访问中心的监听地址，例如 `http://hub.internal.example:8080`。

## 启动中心

```bash
install -d -m 700 ~/.local/state/gpu-monitor
chmod +x ~/.local/share/gpu-monitor/gpu-hub

~/.local/share/gpu-monitor/gpu-hub serve \
  --listen 0.0.0.0:8080 \
  --data ~/.local/state/gpu-monitor
```

首次启动会交互设置 `admin` 和 `viewer` 密码。不要把密码写进启动参数、脚本或仓库。

登录管理界面后创建节点，并复制一次性接入码。接入码 15 分钟内有效，只能兑换一次。

## 接入 GPU 节点

```bash
chmod +x ~/.local/share/gpu-monitor/gpu-agent

~/.local/share/gpu-monitor/gpu-agent setup \
  --server http://hub.internal.example:8080

~/.local/share/gpu-monitor/gpu-agent doctor
~/.local/share/gpu-monitor/gpu-agent start
```

`setup` 会交互读取接入码。不要把接入码放在命令行中，以免进入 shell 历史或进程列表。

## 用户级 systemd 服务

支持 systemd 用户服务的机器可以运行：

```bash
~/.local/share/gpu-monitor/gpu-agent service install --user
systemctl --user status gpu-agent
journalctl --user -u gpu-agent -n 50
```

当 `loginctl show-user "$USER" -p Linger` 返回 `Linger=no` 时，用户退出登录后服务可能停止。程序不会自动修改 linger；是否启用应由系统管理员决定。

## 网络与 TLS

- 只允许受信任的 GPU 节点访问中心上报端口。
- 当前程序不直接终止 TLS。跨不可信网络时，请使用 VPN 或在中心前部署 HTTPS 反向代理。
- 反向代理应保留请求体大小限制和合理的读取、写入超时。
- GPU 节点无需开放入站端口，也无需配置 SSH 反向隧道。

## 升级与备份

升级前停止旧进程，再替换二进制并重新启动。不要直接覆盖仍在运行的程序。

备份中心时应先正常停止 `gpu-hub`，然后复制完整数据目录。恢复后确认目录仍归运行用户所有，并保持权限仅对该用户开放。

## 发布前脱敏清单

- 内网与公网 IP、SSH 地址和端口
- 系统用户名、主机名、节点 ID 和 GPU UUID
- 接入码、节点凭据、密码、Cookie 和 CSRF Token
- 数据库、日志、浏览器下载内容和 `.runtime` 文件
- 包含真实设备或用户信息的截图
