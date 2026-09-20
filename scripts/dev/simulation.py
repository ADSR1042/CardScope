"""Isolated 20-node / 160-GPU UI fixture, never mixed with real WSL data."""

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
state = Path("/tmp/gpu-monitor-simulation-" + secrets.token_hex(4))
state.mkdir(mode=0o700)
os.umask(0o077)
password = secrets.token_urlsafe(20)
log = (state / "hub.log").open("w")
binary = state / "gpu-hub"
shutil.copy2(root / "dist/linux-amd64/gpu-hub", binary)
binary.chmod(0o755)
hub = subprocess.Popen(
    [str(binary), "serve", "--listen", "0.0.0.0:8081", "--data", str(state / "data")],
    stdin=subprocess.PIPE,
    stdout=log,
    stderr=log,
    start_new_session=True,
    text=True,
)
hub.stdin.write(password + "\n" + password + "\n")
hub.stdin.close()
jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(
    urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar)
)
csrf = ""


def request(path, data=None, credential=""):
    headers = {"Content-Type": "application/json"}
    if csrf:
        headers["X-CSRF-Token"] = csrf
    if credential:
        headers["Authorization"] = "Bearer " + credential
    req = urllib.request.Request(
        "http://127.0.0.1:8081/api/v1" + path,
        data=json.dumps(data).encode() if data is not None else None,
        headers=headers,
    )
    with opener.open(req, timeout=15) as r:
        return json.load(r)


for _ in range(40):
    try:
        csrf = request("/login", {"username": "admin", "password": password})["csrf"]
        break
    except Exception:
        time.sleep(0.2)
else:
    raise SystemExit("Simulation hub failed to start")

nodes = []
for i in range(20):
    n = request("/nodes", {"name": f"SIM · training-{i + 1:02d}"})
    auth = request("/enroll", {"code": n["code"]})
    nodes.append(auth)
base = int(time.time() * 1000) - 10000
for seq in range(1, 4):
    for i, n in enumerate(nodes):
        gpus = []
        for j in range(8):
            idle = (i + j) % 4 == 0
            fault = i == 2 and j == 7
            process = (
                []
                if idle or fault
                else [
                    {
                        "pid": 200 + j,
                        "uid": str(1000 + i % 4),
                        "user": ["alice", "bob", "carol", "dave"][i % 4],
                        "name": "python",
                        "created": base - 100000,
                        "memory": (20 + j) * 1024**3,
                        "first_seen": base - 100000,
                        "last_seen": base + seq * 5000,
                    }
                ]
            )
            gpus.append(
                {
                    "uuid": f"SIM-GPU-{i:02d}-{j:02d}",
                    "name": "NVIDIA A100 80GB" if i % 2 == 0 else "NVIDIA RTX 6000 Ada",
                    "index": j,
                    "pci": f"0000:{j + 1:02d}:00.0",
                    "util": None if fault else 0 if idle else 60 + j * 4,
                    "memory_used": None if fault else (0.1 if idle else 20 + j) * 1024**3,
                    "memory_total": (80 if i % 2 == 0 else 48) * 1024**3,
                    "temperature": None if fault else 31 if idle else 57 + j,
                    "power": None if fault else 45 if idle else 230 + j * 8,
                    "ecc": None,
                    "status": "timeout" if fault else "ok",
                    "fields": {"power": "unsupported" if fault else "ok"},
                    "processes": process,
                    "process_status": "unknown" if fault else "ok",
                    "source": "simulation",
                    "seen_at": base + (seq - 1) * 5000,
                }
            )
        data = {
            "version": 1,
            "node_id": n["node_id"],
            "boot_id": "simulation",
            "seq": seq,
            "at": base + (seq - 1) * 5000,
            "interval": 5,
            "hostname": f"sim-node-{i + 1:02d}",
            "system": {
                "cpu": 22 + i,
                "cores": 64,
                "load": 4.2,
                "memory_total": 256 * 1024**3,
                "memory_available": 128 * 1024**3,
                "swap_total": 0,
                "swap_used": 0,
                "networks": [
                    {
                        "name": "bond0",
                        "rx": 1000000 + seq * 5000,
                        "tx": 100000,
                        "rx_rate": 45000000,
                        "tx_rate": 13000000,
                        "up": True,
                        "primary": True,
                    }
                ],
                "disks": [
                    {
                        "path": "/data",
                        "device": "/dev/nvme0n1",
                        "total": 4 * 1024**4,
                        "free": 2 * 1024**4,
                        "status": "ok",
                    }
                ],
                "io": [],
                "wsl": False,
                "errors": {},
            },
            "gpus": gpus,
            "gpu_status": "ok",
            "cache_dropped": 0,
            "backfill": False,
        }
        request("/snapshots", data, n["token"])
(root / ".runtime/simulation-access.json").write_text(
    json.dumps(
        {"url": "http://localhost:8081", "admin": password, "pid": hub.pid, "state": str(state)}
    )
)
print(
    json.dumps(
        {
            "simulation_nodes": 20,
            "simulation_gpus": 160,
            "url": "http://localhost:8081",
            "pid": hub.pid,
        }
    )
)
