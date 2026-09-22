"""What a person gets when they build the company host from this checkout and run it.

The compose stack this file used to test is gone: `internkim install` writes
systemd units and a private state directory where it used to write
`compose.yaml` and `compose.env`, so there is no `docker compose config` left to
read the pair back. What is worth keeping is the end of that build — that the
binary `make` produces is the command a person types, and that the first thing
it does when they type it without privilege is say so and write nothing.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


repository_root = Path(__file__).resolve().parents[2]
built_binary = repository_root / "internkim-host"


def connection_document():
    return {
        "schemaVersion": 1,
        "appURL": "https://company.example.com",
        "company": {"id": "00000000-0000-4000-8000-000000000001", "name": "Example Co", "slug": "example"},
        "centralPlane": {"projectURL": "https://project.supabase.co", "publishableKey": "publishable"},
        "gatewayURL": "wss://gateway.example.com",
        "agentKey": "a" * 64,
    }


class CompanyHostSourceBuildTests(unittest.TestCase):
    def build(self):
        self.addCleanup(built_binary.unlink, missing_ok=True)
        subprocess.run(
            ["make", "build-company-host"],
            cwd=repository_root,
            check=True,
            capture_output=True,
            text=True,
        )
        return built_binary

    def run_install(self):
        binary = self.build()
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        connection = directory / "internkim-host.json"
        connection.write_text(json.dumps(connection_document()))
        model_key = directory / "model-key"
        model_key.write_text("sk-or-example\n")
        state = directory / "state"
        completed = subprocess.run(
            [
                str(binary), "install", str(connection),
                "--state-directory", str(state),
                "--model-key-file", str(model_key),
            ],
            capture_output=True,
            text=True,
        )
        return completed, state

    def test_the_build_produces_the_command_the_package_installs(self):
        binary = self.build()
        completed = subprocess.run([str(binary)], capture_output=True, text=True)
        self.assertEqual(completed.returncode, 1)
        self.assertIn("install <internkim-host.json>", completed.stderr)

    @unittest.skipIf(os.geteuid() == 0, "root would install a company on this machine rather than refuse")
    def test_an_install_without_privilege_refuses_before_writing_anything(self):
        completed, state = self.run_install()
        self.assertEqual(completed.returncode, 1)
        self.assertIn("needs root", completed.stderr)
        self.assertIn("sudo", completed.stderr)
        self.assertFalse(state.exists(), "the refusal left a half-written company behind")


if __name__ == "__main__":
    unittest.main()
