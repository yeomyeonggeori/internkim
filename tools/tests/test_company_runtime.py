import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


class CompanyRuntimeTests(unittest.TestCase):
    def test_host_packages_graphiti_and_preserves_relay_import_layout(self):
        repository_root = Path(__file__).resolve().parents[2]
        dockerfile = (repository_root / "host/Dockerfile").read_text()
        entrypoint = (repository_root / "host/entrypoint.sh").read_text()
        self.assertIn("python3-venv", dockerfile)
        self.assertIn("/opt/internkim/graphiti_memoryd", dockerfile)
        self.assertIn("/opt/internkim/host/relay", dockerfile)
        self.assertIn("/opt/internkim/web/src/lib/person-name.ts", dockerfile)
        self.assertIn("/usr/local/bin/graphiti-memoryd /usr/local/bin/entrypoint.sh", dockerfile)
        self.assertIn("/opt/internkim/graphiti_memoryd/main.py", dockerfile)
        self.assertIn("BLUECLAW_GRAPHITI_KUZU_PATH=", entrypoint)
        self.assertIn("graphiti-memoryd", entrypoint)

    def test_rendered_runtime_carries_graphiti_defaults(self):
        repository_root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary_path = Path(temporary_directory)
            capabilityd_path = temporary_path / "capabilityd"
            capabilityd_path.write_text(
                "#!/bin/sh\n"
                "case \"$1\" in\n"
                "--print-capabilities) printf '{\"tools\":[]}' ;;\n"
                "--print-model-ladder) printf '{\"providers\":[]}' ;;\n"
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
                "GRAPHITI_ENDPOINT": "http://127.0.0.1:8877",
                "GRAPHITI_KUZU_PATH": "/data/company/kuzu",
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
            "adminAssertionKeyPath": "/secrets/agent-key",
            "graphitiEndpoint": "http://127.0.0.1:7791",
            "graphitiKuzuPath": "/workspace/.blueclaw/graphiti/kuzu",
            "timeoutSecond": 30,
        })
        self.assertEqual(custom_runtime["memory"]["adminAssertionKeyPath"], "/run/company/agent-key")
        self.assertEqual(custom_runtime["memory"]["graphitiEndpoint"], "http://127.0.0.1:8877")
        self.assertEqual(custom_runtime["memory"]["graphitiKuzuPath"], "/data/company/kuzu")


if __name__ == "__main__":
    unittest.main()
