#!/usr/bin/env python3
"""User-cron watchdog: start a stopped monitor without requiring a login session."""

import fcntl
import json
import os
import pathlib
import subprocess
import sys


def ensure_running(config_path):
    config_path = pathlib.Path(config_path)
    # Cron may invoke the reboot and periodic checks together; serialize them.
    with config_path.with_suffix(".lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        config = json.loads(config_path.read_text())
        state = pathlib.Path(config["state"])
        name = config["name"]
        pidfile = state / (name + ".pid")
        binary = config["argv"][0]
        try:
            pid = int(pidfile.read_text())
            running = os.readlink("/proc/" + str(pid) + "/exe")
            # Permit an older release during an upgrade, but never signal it.
            releases = str(pathlib.Path(binary).parent.parent) + "/"
            if running.startswith(releases) and pathlib.Path(running).name == name:
                return
        except (FileNotFoundError, ProcessLookupError, ValueError):
            pass
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


if __name__ == "__main__":
    os.umask(0o077)
    ensure_running(sys.argv[1])
