import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


class CompanyRuntimeTests(unittest.TestCase):
    def test_host_carries_no_memory_sidecar_and_preserves_relay_import_layout(self):
        repository_root = Path(__file__).resolve().parents[2]
        dockerfile = (repository_root / "host/Dockerfile").read_text()
        entrypoint = (repository_root / "host/entrypoint.sh").read_text()
        compose = (repository_root / "host/docker-compose.yml").read_text()
        self.assertNotIn("graphiti", dockerfile)
        self.assertNotIn("graphiti", entrypoint)
        self.assertIn("pgvector/pgvector:pg17", compose)
        self.assertIn("/opt/internkim/host/relay", dockerfile)
        self.assertIn("/opt/internkim/web/src/lib/person-name.ts", dockerfile)
        self.assertIn("install -d -o root -g root -m 0700 /root/.internkim", entrypoint)
        self.assertIn('ADMIN_ASSERTION_KEY_PATH="${blueclawAgentKeyPath}"', entrypoint)
        self.assertIn('install -o root -g blueclaw -m 0440 "${agentKeyPath}" "${blueclawAgentKeyPath}"', entrypoint)
        self.assertIn("./secrets:/root/.internkim/secrets:ro", compose)
        self.assertNotIn("./secrets:/secrets:ro", compose)

    def test_rendered_runtime_embeds_with_the_ladders_model(self):
        repository_root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary_path = Path(temporary_directory)
            capabilityd_path = temporary_path / "capabilityd"
            capabilityd_path.write_text(
                "#!/bin/sh\n"
                "case \"$1\" in\n"
                "--print-capabilities) printf '{\"tools\":[]}' ;;\n"
                "--print-model-ladder) printf '{\"embedding\":{\"model\":\"example/embedding\"}}' ;;\n"
                "*) exit 1 ;;\n"
                "esac\n"
            )
            capabilityd_path.chmod(capabilityd_path.stat().st_mode | stat.S_IXUSR)
            output_path = temporary_path / "runtime.json"
            environment = os.environ | {
                "DATABASE_URL": "postgres://example",
                "MESSENGER_PLATFORM": "buzz",
            }
            subprocess.run(
                [
                    str(repository_root / "tools/render-company-runtime"),
                    "--template",
                    str(repository_root / "host/runtime.template.json"),
                    "--capabilityd",
                    str(capabilityd_path),
                    "--out",
                    str(output_path),
                    "--work",
                    str(temporary_path / "work"),
                ],
                check=True,
                env=environment,
                capture_output=True,
                text=True,
            )
            runtime = json.loads(output_path.read_text())
            custom_output_path = temporary_path / "custom-runtime.json"
            custom_environment = environment | {
                "ADMIN_ASSERTION_KEY_PATH": "/run/company/agent-key",
            }
            subprocess.run(
                [
                    str(repository_root / "tools/render-company-runtime"),
                    "--template",
                    str(repository_root / "host/runtime.template.json"),
                    "--capabilityd",
                    str(capabilityd_path),
                    "--out",
                    str(custom_output_path),
                    "--work",
                    str(temporary_path / "custom-work"),
                ],
                check=True,
                env=custom_environment,
                capture_output=True,
                text=True,
            )
            custom_runtime = json.loads(custom_output_path.read_text())

        self.assertEqual(runtime["memory"], {
            "adminAssertionKeyPath": "/root/.internkim/secrets/agent-key",
            "embeddingModel": "example/embedding",
            "embeddingExecutionMode": "auto",
            "extractionDisabled": False,
        })
        self.assertEqual(custom_runtime["memory"]["adminAssertionKeyPath"], "/run/company/agent-key")


if __name__ == "__main__":
    unittest.main()
