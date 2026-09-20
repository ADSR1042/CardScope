#!/usr/bin/env python3
"""User-cron watchdog: start a stopped monitor without requiring a login session."""

import fcntl
import json
import os
import pathlib
import signal
import subprocess
import sys
import time


def running_pid(pidfile, binary):
    try:
        pid = int(pidfile.read_text())
        running = os.readlink("/proc/" + str(pid) + "/exe")
        releases = str(pathlib.Path(binary).parent.parent) + "/"
        if running.startswith(releases) and pathlib.Path(running).name == pathlib.Path(binary).name:
            return pid
    except (FileNotFoundError, ProcessLookupError, ValueError):
        pass
    return None


def ensure_running(config_path, action="start"):
    config_path = pathlib.Path(config_path)
    # Cron may invoke the reboot and periodic checks together; serialize them.
    with config_path.with_suffix(".lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        config = json.loads(config_path.read_text())
        state = pathlib.Path(config["state"])
        name = config["name"]
        pidfile = state / (name + ".pid")
        binary = config["argv"][0]
        pid = running_pid(pidfile, binary)
        if action == "status":
            print("running" if pid else "stopped")
            return 0 if pid else 1
        if action == "stop":
            if pid:
                os.kill(pid, signal.SIGTERM)
                deadline = time.monotonic() + 30
                while running_pid(pidfile, binary):
                    if time.monotonic() >= deadline:
                        raise RuntimeError("Process did not stop; refusing to replace its binary")
                    time.sleep(0.1)
            pidfile.unlink(missing_ok=True)
            return 0
        maintenance = config.get("maintenance")
        if maintenance and pathlib.Path(maintenance).exists():
            return 0
        if pid:
            return 0
        env = os.environ.copy()
        env.update(config.get("env", {}))
        with (state / (name + ".log")).open("a") as log:
            process = subprocess.Popen(
                config["argv"],
                env=env,
                cwd=str(state),
                stdin=subprocess.DEVNULL,
                stdout=log,
                stderr=log,
                start_new_session=True,
            )
        temporary = pidfile.with_suffix(".pid.tmp")
        temporary.write_text(str(process.pid))
        temporary.replace(pidfile)
        time.sleep(1)
        if process.poll() is not None:
            raise RuntimeError("Service exited during startup; inspect its log")
        return 0


if __name__ == "__main__":
    os.umask(0o077)
    action = sys.argv[2] if len(sys.argv) > 2 else "start"
    if action not in ("start", "stop", "status"):
        raise SystemExit("Expected start, stop or status")
    raise SystemExit(ensure_running(sys.argv[1], action))
