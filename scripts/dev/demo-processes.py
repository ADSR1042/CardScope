"""Publish explicitly simulated GPU/process data from WSL for 30 minutes.

No GPU workloads or fake OS processes are created. The demo has its own node,
UUIDs and user names so real collector data and user statistics remain separate.
"""

import http.cookiejar
import json
import math
import os
import secrets
import time
import urllib.request
from pathlib import Path

os.umask(0o077)
state = Path("/tmp/gpu-monitor-local")
access = json.loads((state / "access.json").read_text())
opener = urllib.request.build_opener(
    urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
)
csrf = ""


def request(path, data=None, token=""):
    headers = {"Content-Type": "application/json"}
    if csrf:
        headers["X-CSRF-Token"] = csrf
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(
        "http://127.0.0.1:8080/api/v1" + path,
        data=json.dumps(data).encode() if data is not None else None,
        headers=headers,
    )
    with opener.open(req, timeout=10) as response:
        return json.load(response)


csrf = request("/login", {"username": "admin", "password": access["admin"]})["csrf"]
node = request("/nodes", {"name": "模拟演示 · GPU 进程（非真实负载）"})
auth = request("/enroll", {"code": node["code"]})
request("/logout", {})
csrf = ""
start = int(time.time() * 1000)
boot = secrets.token_hex(16)
print(
    json.dumps({"node_id": auth["node_id"], "duration_minutes": 30, "pid": os.getpid()}), flush=True
)
GiB = 1024**3


def process(pid, uid, user, name, memory, at):
    return dict(
        pid=pid,
        uid=str(uid),
        user=user,
        name=name,
        memory=memory * GiB,
        created=start,
        first_seen=start,
        last_seen=at,
    )


for seq in range(1, 361):
    at = int(time.time() * 1000)
    groups = [
        [
            process(21001, 91001, "demo-alice", "python", 12, at),
            process(21002, 91001, "demo-alice", "python", 6, at),
        ],
        [
            process(22001, 91002, "demo-bob", "python", 10, at),
            process(22002, 91003, "demo-carol", "llama-server", 8, at),
        ],
        [process(23001, 91001, "demo-alice", "python", 5, at)],
        [],
    ]
    gpus = []
    for index, processes in enumerate(groups):
        util = round(
            [83, 57, 24, 0][index] + (3 * math.sin(seq / 3 + index) if processes else 0), 1
        )
        gpus.append(
            dict(
                uuid=f"SIM-{auth['node_id']}-{index}",
                name="NVIDIA RTX 4090（模拟）",
                index=index,
                pci=f"0000:0{index + 1}:00.0",
                util=util,
                memory_used=sum(p["memory"] for p in processes) + 200 * 1024**2,
                memory_total=24 * GiB,
                temperature=round(32 + util * 0.4),
                power=round(25 + util * 3),
                ecc=None,
                status="ok",
                fields={"data": "simulated"},
                processes=processes,
                process_status="ok",
                source="simulation",
                seen_at=at,
            )
        )
    snapshot = dict(
        version=1,
        node_id=auth["node_id"],
        boot_id=boot,
        seq=seq,
        at=at,
        interval=5,
        hostname="SIMULATED-PROCESSES",
        gpu_status="ok",
        gpus=gpus,
        backfill=False,
        cache_dropped=0,
        system=dict(
            cpu=34,
            cores=32,
            load=5.2,
            memory_total=128 * GiB,
            memory_available=80 * GiB,
            swap_total=0,
            swap_used=0,
            networks=[],
            disks=[],
            io=[],
            wsl=False,
            errors={},
        ),
    )
    try:
        request("/snapshots", snapshot, auth["token"])
    except Exception as error:
        print(type(error).__name__, flush=True)
    time.sleep(5)
print("Demo finished; node will become offline.", flush=True)
