"""Isolated user installer tests; cron is faked, services never access the network."""

import json
import os
import shutil
import sqlite3
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


class HubInitTest(unittest.TestCase):
    def test_init_preserves_accounts_on_repeat(self):
        binary = os.environ.get("CARDSCOPE_HUB_BINARY")
        if not binary:
            self.skipTest("CARDSCOPE_HUB_BINARY not provided")
        with tempfile.TemporaryDirectory(prefix="cardscope-init-") as data:
            args = [binary, "init", "--data", data]
            first = subprocess.run(
                args,
                input="synthetic-admin-password\nsynthetic-viewer-password\n",
                text=True,
                capture_output=True,
                timeout=15,
            )
            self.assertEqual(first.returncode, 0, first.stderr)
            with sqlite3.connect(Path(data) / "monitor.db") as db:
                accounts = db.execute("SELECT * FROM accounts ORDER BY username").fetchall()
            self.assertEqual(len(accounts), 2)
            second = subprocess.run(args, input="", text=True, capture_output=True, timeout=15)
            self.assertEqual(second.returncode, 0, second.stderr)
            with sqlite3.connect(Path(data) / "monitor.db") as db:
                self.assertEqual(
                    accounts, db.execute("SELECT * FROM accounts ORDER BY username").fetchall()
                )


@unittest.skipUnless(os.name == "posix", "Linux only")
class InstallTest(unittest.TestCase):
    def setUp(self):
        if os.getuid() == 0:
            self.skipTest("run as a normal user")
        self.temp = tempfile.TemporaryDirectory(prefix="cardscope-install-")
        self.root = Path(self.temp.name)
        self.release = self.root / "release"
        self.release.mkdir()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.cron = self.root / "crontab"
        self.cron.write_text("# unrelated user task\n")
        for name in ("install-hub.sh", "install-agent.sh", "install-user.sh", "ensure-running.py"):
            shutil.copyfile(ROOT / "scripts/ops" / name, self.release / name)
        for role in ("hub", "agent"):
            shutil.copyfile(os.environ["CARDSCOPE_TEST_BINARY"], self.release / f"gpu-{role}")
        self.env = dict(os.environ, HOME=str(self.root), TEST_CRON=str(self.cron))
        self.env.pop("GPU_AGENT_HOME", None)
        self.env.pop("XDG_CONFIG_HOME", None)
        self.env["PATH"] = str(self.bin) + os.pathsep + os.environ["PATH"]
        self.stub("pgrep", 'test "${NO_CRON:-}" != yes\n')
        self.stub(
            "crontab", 'if [ "$1" = -l ]; then cat "$TEST_CRON"; else cp "$1" "$TEST_CRON"; fi\n'
        )
        self.addCleanup(self.cleanup)

    def stub(self, name, script):
        path = self.bin / name
        path.write_text("#!/bin/sh\nset -eu\n" + script)
        path.chmod(0o755)

    def base(self, role):
        return self.root / ".local/share/cardscope" / role

    def cleanup(self):
        for role in ("hub", "agent"):
            base = self.base(role)
            config = base / "watchdog.json"
            if config.exists():
                subprocess.run(
                    ["python3", str(base / "ensure-running.py"), str(config), "stop"],
                    check=True,
                    timeout=40,
                )
        self.temp.cleanup()

    def install(self, role, *args, success=True, **env):
        result = subprocess.run(
            ["bash", str(self.release / f"install-{role}.sh"), *args],
            env=dict(self.env, **env),
            text=True,
            capture_output=True,
            timeout=45,
        )
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        return result

    def test_both_roles_upgrade_without_duplicate_cron_or_losing_data(self):
        for role, args in (("hub", []), ("agent", ["http://127.0.0.1:8080"])):
            self.install(role, *args)
            base = self.base(role)
            data = Path((base / "data-path").read_text().strip())
            (data / "sentinel").write_text("keep")
            old = (base / "current").resolve()
            self.install(role)
            self.assertNotEqual(old, (base / "current").resolve())
            self.assertEqual((data / "sentinel").read_text(), "keep")
            self.assertTrue(list((base / "backups").glob("*.tar.gz")))
            self.assertEqual(self.cron.read_text().count(f"# CardScope {role}"), 2)
        self.assertIn("# unrelated user task", self.cron.read_text())

    def test_existing_hub_data_is_preserved(self):
        old = self.root / "old-data"
        old.mkdir()
        (old / "monitor.db").write_text("existing")
        self.install("hub", str(old))
        self.assertEqual((old / "monitor.db").read_text(), "existing")
        self.assertEqual((self.base("hub") / "data-path").read_text().strip(), str(old))

    def test_requires_cron_before_installing(self):
        self.install("hub", success=False, NO_CRON="yes")
        self.assertFalse(self.base("hub").exists())

    def test_backup_failure_restarts_old_service(self):
        self.install("hub")
        base = self.base("hub")
        old = (base / "current").resolve()
        self.stub("tar", "exit 1\n")
        self.install("hub", success=False)
        self.assertEqual((base / "current").resolve(), old)
        self.assertFalse((base / "maintenance").exists())
        subprocess.run(
            ["python3", str(base / "ensure-running.py"), str(base / "watchdog.json"), "status"],
            check=True,
        )

    def test_start_failure_pauses_watchdog_and_preserves_backup(self):
        self.install("agent", "http://127.0.0.1:8080")
        (self.release / "gpu-agent").write_text("#!/bin/sh\nexit 1\n")
        self.install("agent", success=False)
        base = self.base("agent")
        self.assertTrue((base / "maintenance").exists())
        self.assertTrue(list((base / "backups").glob("*.tar.gz")))
        config = json.loads((base / "watchdog.json").read_text())
        self.assertTrue((Path(config["state"]) / "config.json").exists())


if __name__ == "__main__":
    unittest.main()
