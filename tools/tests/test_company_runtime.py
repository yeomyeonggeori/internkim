import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


class CompanyRuntimeTests(unittest.TestCase):
    def test_rendered_runtime_embeds_with_the_ladders_model_and_width(self):
        repository_root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary_path = Path(temporary_directory)
            capabilityd_path = temporary_path / "capabilityd"
            capabilityd_path.write_text(
                "#!/bin/sh\n"
                "case \"$1\" in\n"
                "--print-capabilities) printf '{\"tools\":[]}' ;;\n"
                "--print-model-ladder) printf '{\"embedding\":{\"model\":\"example/embedding\",\"dimensions\":768}}' ;;\n"
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
            "embeddingDimensions": 768,
            "embeddingExecutionMode": "auto",
            "extractionDisabled": False,
        })
        self.assertEqual(custom_runtime["memory"]["adminAssertionKeyPath"], "/run/company/agent-key")

    def test_a_database_address_with_sed_metacharacters_arrives_intact(self):
        """The host reaches its database on a socket, so its address carries a query:
        `?host=%2Frun%2Finternkim-postgres&sslmode=...`. The renderer once substituted
        with sed, where `&` in the replacement means the text that matched."""
        repository_root = Path(__file__).resolve().parents[2]
        address = "postgres://u:p@localhost/db?host=%2Frun%2Finternkim-postgres&a=b|c\\d"
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary_path = Path(temporary_directory)
            capabilityd_path = temporary_path / "capabilityd"
            capabilityd_path.write_text(
                "#!/bin/sh\n"
                "case \"$1\" in\n"
                "--print-capabilities) printf '{\"tools\":[]}' ;;\n"
                "--print-model-ladder) printf '{\"embedding\":{\"model\":\"example/embedding\",\"dimensions\":768}}' ;;\n"
                "*) exit 1 ;;\n"
                "esac\n"
            )
            capabilityd_path.chmod(capabilityd_path.stat().st_mode | stat.S_IXUSR)
            output_path = temporary_path / "runtime.json"
            subprocess.run(
                [
                    str(repository_root / "tools/render-company-runtime"),
                    "--template", str(repository_root / "host/runtime.template.json"),
                    "--capabilityd", str(capabilityd_path),
                    "--out", str(output_path),
                    "--work", str(temporary_path / "work"),
                ],
                check=True,
                env=os.environ | {"DATABASE_URL": address, "MESSENGER_PLATFORM": "buzz"},
                capture_output=True,
                text=True,
            )
            runtime = json.loads(output_path.read_text())
        self.assertEqual(runtime["database"]["connectionString"], address)

    def test_the_work_directory_keeps_nothing_but_the_runtime_document(self):
        """The host renders into its run directory, which the relay's account may
        pass through, and the rendered template carries the database password."""
        repository_root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary_path = Path(temporary_directory)
            capabilityd_path = temporary_path / "capabilityd"
            capabilityd_path.write_text(
                "#!/bin/sh\n"
                "case \"$1\" in\n"
                "--print-capabilities) printf '{\"tools\":[]}' ;;\n"
                "--print-model-ladder) printf '{\"embedding\":{\"model\":\"example/embedding\",\"dimensions\":768}}' ;;\n"
                "*) exit 1 ;;\n"
                "esac\n"
            )
            capabilityd_path.chmod(capabilityd_path.stat().st_mode | stat.S_IXUSR)
            run_directory = temporary_path / "run"
            subprocess.run(
                [
                    str(repository_root / "tools/render-company-runtime"),
                    "--template", str(repository_root / "host/runtime.template.json"),
                    "--capabilityd", str(capabilityd_path),
                    "--out", str(run_directory / "runtime.json"),
                    "--work", str(run_directory),
                ],
                check=True,
                env=os.environ | {"DATABASE_URL": "postgres://u:secret@localhost/db", "MESSENGER_PLATFORM": "buzz"},
                capture_output=True,
                text=True,
            )
            left_behind = sorted(entry.name for entry in run_directory.iterdir())
        self.assertEqual(left_behind, ["runtime.json"])

    def test_the_ladder_is_asked_for_the_model_route_with_the_service_token(self):
        """On a central plane the ladder reaches capabilityd's model route, which
        follows whoever pays while the services run, so the renderer must hand
        capabilityd the route and the token the route asks for."""
        repository_root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary_directory:
            temporary_path = Path(temporary_directory)
            asked_path = temporary_path / "ladder-arguments"
            capabilityd_path = temporary_path / "capabilityd"
            capabilityd_path.write_text(
                "#!/bin/sh\n"
                "case \"$1\" in\n"
                "--print-capabilities) printf '{\"tools\":[]}' ;;\n"
                f"--print-model-ladder) echo \"$@\" > {asked_path}; printf '{{\"embedding\":{{\"model\":\"example/embedding\",\"dimensions\":768}}}}' ;;\n"
                "*) exit 1 ;;\n"
                "esac\n"
            )
            capabilityd_path.chmod(capabilityd_path.stat().st_mode | stat.S_IXUSR)
            subprocess.run(
                [
                    str(repository_root / "tools/render-company-runtime"),
                    "--template", str(repository_root / "host/runtime.template.json"),
                    "--capabilityd", str(capabilityd_path),
                    "--out", str(temporary_path / "runtime.json"),
                    "--work", str(temporary_path / "work"),
                ],
                check=True,
                env=os.environ | {
                    "DATABASE_URL": "postgres://example",
                    "MESSENGER_PLATFORM": "buzz",
                    "SUPABASE_URL": "https://plane.example",
                },
                capture_output=True,
                text=True,
            )
            asked = asked_path.read_text().split()
        self.assertEqual(asked[asked.index("--model-gateway") + 1], "https://plane.example")
        self.assertEqual(asked[asked.index("--model-route") + 1], "http://127.0.0.1:18093")
        self.assertEqual(asked[asked.index("--model-route-key") + 1], "/run/internkim/secrets/service-token")


if __name__ == "__main__":
    unittest.main()
