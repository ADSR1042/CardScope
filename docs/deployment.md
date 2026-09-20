# 安装与升级

服务端和客户端都使用普通用户运行，不需要 sudo。发行包内有两个入口，
`install-hub.sh` 和 `install-agent.sh`。解压发行包后，在其目录中执行。

机器需要 Bash、Python 3.8+、用户 crontab、flock，以及已经运行的 cron/crond
服务。脚本写入当前用户的 `@reboot` 和每分钟守护任务，保留其他 cron 任务。
进程退出后会在下一分钟重启。安装时缺少 cron 会报错，不会宣称自启成功。
如果管理员禁用了用户 cron，或者用户目录开机时不可访问，需要管理员先处理；
普通用户脚本无法自行绕过这些系统限制。

## 服务端

首次新建服务端，执行后输入管理员和只读账号密码：

```bash
bash install-hub.sh
```

默认监听 `0.0.0.0:8080`，浏览器打开 `http://服务端IP:8080`。
数据保存在 `~/.local/share/cardscope/hub/data`。

已有服务端首次迁移到这套脚本时，先停止原进程并取消原来的 cron、systemd
或其他守护任务，再传入**原来 `--data` 指向的目录**：

```bash
bash install-hub.sh /原服务端数据目录
```

原账号、节点和历史数据会保留。脚本记录此目录，后续不要再传目录。
迁移和升级旧数据库时需要 `fuser`，一般由系统的 psmisc 包提供。
当前固定使用 8080 端口；原部署若使用其他端口，需要先调整接入地址或转发配置。

## 客户端

先在服务端网页添加节点并生成接入码，然后在对应 GPU 机器上执行：

```bash
bash install-agent.sh http://服务端IP:8080
```

按提示输入接入码，脚本会启动采集并设置自启。接入码不放在命令行。
客户端仍需机器已有 NVIDIA 驱动，并允许当前用户读取 GPU。

已有客户端先停止旧进程及旧守护任务。脚本默认复用
`${XDG_CONFIG_HOME:-$HOME/.config}/gpu-monitor`，有配置时不重新接入。
若原部署指定了 `GPU_AGENT_HOME`，首次迁移时保留该环境变量：

```bash
GPU_AGENT_HOME=/原客户端配置目录 bash install-agent.sh
```

## 后续升级

将新发行包解压到另一个目录，再以**原来的同一个用户**运行对应脚本：

```bash
bash install-hub.sh
# 或在客户端机器上
bash install-agent.sh
```

脚本停止自身管理的旧进程，备份完整数据目录，再切换新程序；升级保留账号、
节点凭据和客户端缓存。不重复添加 cron 任务。初次迁移时必须自行停用旧守护，
避免两个守护任务互相启动进程。

程序保存在 `~/.local/share/cardscope/hub` 或 `agent` 下的 `releases/`，
数据备份保存在对应的 `backups/`。程序与备份不会自动清理，请定期检查磁盘空间。
数据已可能迁移时，升级失败会暂停守护，并保留旧程序和备份；修正问题后重跑
脚本。若需要降级，先停止守护，再恢复对应数据备份和旧程序，勿只替换二进制。

## 检查与停止

```bash
crontab -l
base="$HOME/.local/share/cardscope/hub"  # 客户端改为 agent
python3 "$base/ensure-running.py" "$base/watchdog.json" status
```

服务日志在数据目录内的 `gpu-hub.log` 或 `gpu-agent.log`，守护日志在安装目录的
`watchdog.log`。需要临时停机时，先暂停守护，再停止进程：

```bash
touch "$base/maintenance"
python3 "$base/ensure-running.py" "$base/watchdog.json" stop
# 恢复
rm "$base/maintenance"
python3 "$base/ensure-running.py" "$base/watchdog.json"
```

验证真正的开机自启需要在计划维护时重启机器后检查状态；安装脚本不会重启机器。
