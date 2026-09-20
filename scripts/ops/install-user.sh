#!/usr/bin/env bash
# Shared implementation for the two user-mode installers. No sudo or systemd.
set -euo pipefail
umask 077
die() { echo "$*" >&2; exit 1; }
role=${1:-}
[[ $role == hub || $role == agent ]] || die '使用 install-hub.sh 或 install-agent.sh。'
shift
if [[ ${1:-} == --help ]]; then
  if [[ $role == hub ]]; then echo 'bash install-hub.sh [原服务端数据目录]';
  else echo 'bash install-agent.sh [http://服务端IP:8080]'; fi
  exit 0
fi
[[ $(uname -s) == Linux && $EUID != 0 ]] || die '请在 Linux 上使用普通用户执行，不要 sudo。'
[[ $# -le 1 ]] || die '首次安装只需一个参数，升级无需参数。'
for command in python3 crontab flock pgrep; do command -v "$command" >/dev/null || die "需要 $command。"; done
source_dir=$(cd "$(dirname "$0")" && pwd)
[[ -f $source_dir/gpu-$role && -f $source_dir/ensure-running.py ]] || die '请从完整发行包运行此脚本。'
if ! pgrep -x cron >/dev/null && ! pgrep -x crond >/dev/null; then
  die '未检测到 cron 服务，请管理员启用 cron/crond 后再安装。'
fi
base="$HOME/.local/share/cardscope/$role"
valid_path() { [[ $1 == /* && $1 != / && $1 =~ ^[a-zA-Z0-9_./-]+$ ]]; }
valid_path "$base" || die '安装目录须为不含空格的绝对路径。'
mkdir -p "$base"
exec 9>"$base/install.lock"
flock -n 9 || die '另一次安装正在运行。'
if [[ $role == hub ]]; then data=${1:-$base/data};
else data="${GPU_AGENT_HOME:-${XDG_CONFIG_HOME:-$HOME/.config}/gpu-monitor}"; fi
if [[ -f $base/data-path ]]; then
  saved=$(cat "$base/data-path")
  [[ $role != hub || $# == 0 || $data == "$saved" ]] || die '升级不能更换数据目录。'
  data=$saved
  if [[ $role == hub ]]; then required=monitor.db; else required=config.json; fi
  [[ -f $data/$required ]] || die '原数据或配置丢失，拒绝创建空实例。'
fi
valid_path "$data" || die '数据目录须为不含空格的绝对路径。'
mkdir -p "$data"
data=$(cd "$data" && pwd -P)
[[ $base/ != "$data/"* ]] || die '数据目录不能包含安装目录，否则无法安全备份。'
[[ -w $data ]] || die '当前用户不能写入数据目录。'
python_bin=$(command -v python3)
valid_path "$python_bin" || die 'python3 路径不受支持。'
cronfile=$(mktemp)
trap 'rm -f "$cronfile"' EXIT
if ! LC_ALL=C crontab -l > "$cronfile" 2> "$base/cron-error"; then
  grep -qi 'no crontab' "$base/cron-error" || die '无法读取用户 crontab，未进行覆盖。'
  : > "$cronfile"
fi
stamp=$(date -u +%Y%m%dT%H%M%SZ)-$$
release="$base/releases/$stamp"
mkdir -p "$release"
install -m 755 "$source_dir/gpu-$role" "$release/gpu-$role"
if [[ $role == agent ]]; then
  if [[ ! -f $data/config.json ]]; then
    [[ $# == 1 ]] || die '首次安装需指定服务端地址。'
    GPU_AGENT_HOME="$data" "$release/gpu-agent" setup --server "$1"
  fi
  python3 - "$data/config.json" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    c = json.load(f)
interval = c.get('interval')
if not c.get('node_id') or not c.get('token') or not isinstance(interval, int) or not 2 <= interval <= 20:
    raise SystemExit('现有配置无效或采样间隔不在 2～20 秒；请先修正，不会自动覆盖凭据。')
PY
fi
managed=no stopped=no activated=no
[[ ! -f $base/watchdog.json ]] || managed=yes
recover() {
  result=$?
  rm -f "$cronfile"
  if ((result != 0)); then
    if [[ $activated == no && $stopped == yes ]]; then
      rm -f "$base/maintenance"
      python3 "$base/ensure-running.py" "$base/watchdog.json" || true
    elif [[ $activated == yes ]]; then
      touch "$base/maintenance"
      if [[ -f $base/watchdog.json ]]; then
        python3 "$base/ensure-running.py" "$base/watchdog.json" stop || true
      fi
      echo "升级未完成，守护已暂停。旧程序保留在 $base/releases，数据备份位于 $base/backups。" >&2
      echo '检查日志后可重新运行安装脚本。降级前需恢复对应的数据备份。' >&2
    fi
  fi
}
trap recover EXIT
touch "$base/maintenance"
if [[ $managed == yes ]]; then
  python3 "$base/ensure-running.py" "$base/watchdog.json" stop
  stopped=yes
fi
if [[ $role == agent ]]; then
  exec 8>"$data/agent.lock"
  flock -n 8 || die '原客户端仍在运行，请先停止原客户端及其守护任务。'
elif [[ -f $data/monitor.db ]]; then
  command -v fuser >/dev/null || die '备份已有数据库需要 fuser（通常由 psmisc 提供）。'
  if fuser "$data/monitor.db" >/dev/null 2>&1; then
    die '原服务仍在使用数据库，请先停止原服务及其守护任务。'
  fi
fi
# Backups are outside the data directory. Copy SQLite WAL/SHM and agent queue too.
if [[ -f $data/monitor.db || -f $data/config.json ]]; then
  mkdir -p "$base/backups"
  tar -czf "$base/backups/$stamp.tar.gz" -C "$data" .
  echo "数据备份：$base/backups/$stamp.tar.gz"
fi
activated=yes
if [[ $role == hub ]]; then "$release/gpu-hub" init --data "$data"; fi
install -m 700 "$source_dir/ensure-running.py" "$base/ensure-running.py"
ln -s "$release" "$base/current.$stamp"
mv -Tf "$base/current.$stamp" "$base/current"
python3 - "$base" "$data" "$role" <<'PY'
import json, pathlib, sys
base, data = map(pathlib.Path, sys.argv[1:3])
role = sys.argv[3]
argv = [str(base / ('current/gpu-' + role))]
argv += ['serve', '--listen', '0.0.0.0:8080', '--data', str(data)] if role == 'hub' else ['start']
config = {'name': 'gpu-' + role, 'state': str(data), 'argv': argv,
          'env': {'GPU_AGENT_HOME': str(data)} if role == 'agent' else {},
          'maintenance': str(base / 'maintenance')}
(base / 'watchdog.json').write_text(json.dumps(config))
PY
printf '%s\n' "$data" > "$base/data-path"
sed -i "/# CardScope $role\$/d" "$cronfile"
printf '@reboot %s %s/ensure-running.py %s/watchdog.json >> %s/watchdog.log 2>&1 # CardScope %s\n' "$python_bin" "$base" "$base" "$base" "$role" >> "$cronfile"
printf '* * * * * %s %s/ensure-running.py %s/watchdog.json >> %s/watchdog.log 2>&1 # CardScope %s\n' "$python_bin" "$base" "$base" "$base" "$role" >> "$cronfile"
crontab "$cronfile"
flock -u 8 2>/dev/null || true
rm -f "$base/maintenance"
python3 "$base/ensure-running.py" "$base/watchdog.json"
echo "$role 已启动，已添加开机启动和每分钟守护。日志：$data/gpu-$role.log"
