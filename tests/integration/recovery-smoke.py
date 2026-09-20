"""Exercise the real agent's exclusive lock and offline queue against its test hub."""

import json
import os
import signal
import sqlite3
import subprocess
import time
from pathlib import Path

root = Path(__file__).resolve().parents[2]
state = Path("/tmp/gpu-monitor-local")
agent_pid = int((state / "agent.pid").read_text())
hub_pid = int((state / "hub.pid").read_text())
agent_binary = os.readlink(f"/proc/{agent_pid}/exe")
hub_binary = os.readlink(f"/proc/{hub_pid}/exe")
if not agent_binary.startswith(str(state / "bin") + "/") or not hub_binary.startswith(
    str(state / "bin") + "/"
):
    raise SystemExit("Unexpected running executable; refusing to stop it")
env = dict(
    os.environ,
    GPU_AGENT_HOME=str(state / "agent"),
    HTTP_PROXY="",
    HTTPS_PROXY="",
    ALL_PROXY="",
    NO_PROXY="*",
)
second = subprocess.run([agent_binary, "start"], env=env, capture_output=True, text=True, timeout=5)
assert second.returncode != 0 and "重复启动" in second.stderr, second.stderr
os.kill(hub_pid, signal.SIGTERM)
for _ in range(30):
    try:
        stat = Path(f"/proc/{hub_pid}/stat").read_text()
        if stat.split(") ", 1)[1].startswith("Z"):
            break
    except FileNotFoundError:
        break
    time.sleep(0.1)
time.sleep(13)
queued = list((state / "agent" / "queue").glob("*.json"))
assert len(queued) >= 2, f"Expected offline samples, found {len(queued)}"
retained = [json.loads(p.read_text()) for p in queued]
log = (state / "hub.log").open("a")
hub = subprocess.Popen(
    [hub_binary, "serve", "--listen", "0.0.0.0:8080", "--data", str(state / "hub")],
    stdin=subprocess.DEVNULL,
    stdout=log,
    stderr=log,
    start_new_session=True,
)
(state / "hub.pid").write_text(str(hub.pid))
for _ in range(30):
    if all(not p.exists() for p in queued):
        break
    time.sleep(1)
else:
    raise SystemExit("Offline samples did not drain")
db = sqlite3.connect(state / "hub" / "monitor.db")
for s in retained:
    count = db.execute(
        "SELECT count(*) FROM samples WHERE node=? AND boot=? AND seq=?",
        (s["node_id"], s["boot_id"], s["seq"]),
    ).fetchone()[0]
    assert count == 1, "Sample missing or duplicated"
result = {
    "exclusive_lock": True,
    "offline_samples": len(queued),
    "replayed_exactly_once": True,
    "hub_restarted": hub.poll() is None,
}
(root / ".runtime/recovery-result.json").write_text(json.dumps(result, indent=2))
print(json.dumps(result))
