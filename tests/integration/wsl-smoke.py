"""Run the real WSL agent/hub with private temporary state. No sudo, no GPU load."""

import hashlib
import http.cookiejar
import json
import os
import secrets
import shutil
import subprocess
import time
import urllib.request
from pathlib import Path

root = Path(__file__).resolve().parents[2]
state = Path("/tmp/gpu-monitor-local")
state.mkdir(mode=0o700, exist_ok=True)
if state.stat().st_uid != os.getuid():
    raise SystemExit("Unexpected state directory owner")
os.chmod(state, 0o700)
os.umask(0o077)
runtime = root / ".runtime"
runtime.mkdir(exist_ok=True)
source_bins = root / "dist" / "linux-amd64"
version = hashlib.sha256(
    (source_bins / "gpu-agent").read_bytes() + (source_bins / "gpu-hub").read_bytes()
).hexdigest()[:12]
bins = state / "bin" / version
bins.mkdir(parents=True, exist_ok=True)
for name in ["gpu-agent", "gpu-hub"]:
    if not (bins / name).exists():
        shutil.copy2(source_bins / name, bins / name)
    (bins / name).chmod(0o755)

report = subprocess.run(
    [str(bins / "gpu-agent"), "doctor"], capture_output=True, text=True, timeout=30
)
if report.returncode:
    raise SystemExit(report.stderr)
doctor = json.loads(report.stdout)
(runtime / "doctor.json").write_text(json.dumps(doctor, ensure_ascii=False, indent=2))

access_path = state / "access.json"
if access_path.exists():
    access = json.loads(access_path.read_text())
else:
    access = {"admin": secrets.token_urlsafe(20), "viewer": secrets.token_urlsafe(20)}
    access_path.write_text(json.dumps(access))
    access_path.chmod(0o600)

# Only stop processes previously launched by this script and verified by executable.
for name in ["agent", "hub"]:
    pid_path = state / (name + ".pid")
    if pid_path.exists():
        pid = int(pid_path.read_text())
        try:
            target = os.readlink(f"/proc/{pid}/exe")
            if target.startswith(str(source_bins / ("gpu-" + name))) or target.startswith(
                str(state / "bin") + "/"
            ):
                os.kill(pid, 15)
                for _ in range(30):
                    if not Path(f"/proc/{pid}").exists():
                        break
                    time.sleep(0.1)
        except (FileNotFoundError, ProcessLookupError):
            pass

hub_log = (state / "hub.log").open("a")
hub = subprocess.Popen(
    [str(bins / "gpu-hub"), "serve", "--listen", "0.0.0.0:8080", "--data", str(state / "hub")],
    stdin=subprocess.PIPE,
    stdout=hub_log,
    stderr=hub_log,
    start_new_session=True,
    text=True,
)
hub.stdin.write(access["admin"] + "\n" + access["viewer"] + "\n")
hub.stdin.close()
(state / "hub.pid").write_text(str(hub.pid))
jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(
    urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar)
)
csrf = ""


def request(path, body=None):
    headers = {"Content-Type": "application/json"}
    if csrf:
        headers["X-CSRF-Token"] = csrf
    req = urllib.request.Request(
        "http://127.0.0.1:8080/api/v1" + path,
        data=json.dumps(body).encode() if body is not None else None,
        headers=headers,
    )
    with opener.open(req, timeout=10) as res:
        return json.load(res)


for attempt in range(40):
    try:
        user = request("/login", {"username": "admin", "password": access["admin"]})
        csrf = user["csrf"]
        break
    except Exception:
        if hub.poll() is not None:
            raise SystemExit((state / "hub.log").read_text())
        time.sleep(0.5)
else:
    raise SystemExit("Hub startup timed out")

env = dict(
    os.environ,
    GPU_AGENT_HOME=str(state / "agent"),
    HTTP_PROXY="",
    HTTPS_PROXY="",
    ALL_PROXY="",
    NO_PROXY="*",
)
if not (state / "agent" / "config.json").exists():
    node = request("/nodes", {"name": "本机 WSL · GTX 1050"})
    result = subprocess.run(
        [str(bins / "gpu-agent"), "setup", "--server", "http://127.0.0.1:8080"],
        input=node["code"] + "\n",
        capture_output=True,
        text=True,
        env=env,
        timeout=15,
    )
    if result.returncode:
        raise SystemExit(result.stderr)

agent_log = (state / "agent.log").open("a")
started_at = int(time.time() * 1000)
agent = subprocess.Popen(
    [str(bins / "gpu-agent"), "start"],
    env=env,
    stdout=agent_log,
    stderr=agent_log,
    start_new_session=True,
)
(state / "agent.pid").write_text(str(agent.pid))
for _ in range(30):
    agent_node = json.loads((state / "agent" / "config.json").read_text())["node_id"]
    nodes = [n for n in request("/nodes") if n["id"] == agent_node]
    if (
        nodes
        and nodes[0]["online"]
        and nodes[0]["sample_at"] >= started_at
        and nodes[0]["snapshot"]["gpus"]
        and nodes[0]["snapshot"]["gpus"][0]["memory_total"]
    ):
        break
    time.sleep(1)
else:
    raise SystemExit("Real agent did not report: " + (state / "agent.log").read_text())

# Browser test credentials remain ignored local runtime state, never in reports.
(runtime / "access.json").write_text(json.dumps({"url": "http://localhost:8080", **access}))
summary = {
    "uid": os.getuid(),
    "url": "http://localhost:8080",
    "online_nodes": len([n for n in nodes if n["online"]]),
    "gpus": [
        {
            "name": g["name"],
            "source": g["source"],
            "memory_total": g["memory_total"],
            "power": g["power"],
            "process_status": g["process_status"],
        }
        for g in nodes[0]["snapshot"]["gpus"]
    ],
    "config_mode": oct((state / "agent" / "config.json").stat().st_mode & 0o777),
}
(runtime / "wsl-result.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2))
print(json.dumps(summary, ensure_ascii=False, indent=2))
