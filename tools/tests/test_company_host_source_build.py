import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


repository_root = Path(__file__).resolve().parents[2]
built_binary = repository_root / "internkim-host"
image = "internkim-company-host:source-build-test"


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
            ["make", "build-company-host", f"COMPANY_HOST_IMAGE={image}"],
            cwd=repository_root,
            check=True,
            capture_output=True,
            text=True,
        )
        return built_binary

    def docker_shim(self, directory):
        binary_directory = directory / "bin"
        binary_directory.mkdir()
        shim = binary_directory / "docker"
        shim.write_text(
            "#!/bin/sh\n"
            'if [ "$1" = "info" ]; then echo linux; exit 0; fi\n'
            'if [ "$1" = "compose" ] && [ "$2" = "version" ]; then echo v2; exit 0; fi\n'
            f'echo "docker $*" >> "{directory}/docker.log"\n'
        )
        shim.chmod(0o755)
        return str(binary_directory)

    def install_into(self, directory_name="state"):
        binary = self.build()
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        connection = directory / "internkim-host.json"
        connection.write_text(json.dumps(connection_document()))
        model_key = directory / "model-key"
        model_key.write_text("sk-or-example\n")
        state = directory / directory_name
        completed = subprocess.run(
            [
                str(binary), "install", str(connection),
                "--state-directory", str(state),
                "--model-key-file", str(model_key),
            ],
            capture_output=True,
            text=True,
            env={**os.environ, "PATH": self.docker_shim(directory) + os.pathsep + os.environ["PATH"]},
        )
        return completed, directory, state

    def test_a_build_from_source_installs_against_a_locally_built_image(self):
        completed, directory, state = self.install_into()
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertIn(f"HOST_IMAGE='{image}'", (state / "compose.env").read_text())
        self.assertIn("up --detach --wait", (directory / "docker.log").read_text())

    @unittest.skipUnless(shutil.which("docker"), "the docker CLI is what reads the pair this test is about")
    def test_compose_reads_back_every_value_the_installer_wrote(self):
        completed, _, state = self.install_into("state with 'quotes' and $dollars")
        self.assertEqual(completed.returncode, 0, completed.stderr)

        configured = subprocess.run(
            [
                "docker", "compose",
                "--env-file", str(state / "compose.env"),
                "--file", str(state / "compose.yaml"),
                "config",
            ],
            capture_output=True,
            text=True,
            env={**os.environ, "DOCKER_HOST": "unix:///nonexistent"},
        )
        self.assertEqual(configured.returncode, 0, configured.stderr)

        read_back = configured.stdout.replace("$$", "$")
        self.assertIn(str(state), read_back)
        self.assertIn(image, read_back)
        for name in ("postgres-password", "media-access-key", "media-secret-key"):
            kept = (state / "secrets" / name).read_text().strip()
            self.assertIn(kept, read_back)


if __name__ == "__main__":
    unittest.main()
