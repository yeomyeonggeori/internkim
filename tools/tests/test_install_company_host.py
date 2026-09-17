import importlib.machinery
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest


repository_root = Path(__file__).resolve().parents[2]
installer = importlib.machinery.SourceFileLoader(
    "install_company_host", str(repository_root / "tools/install-company-host")
).load_module()


def configuration():
    return {
        "schemaVersion": 1,
        "appURL": "https://company.example.com",
        "company": {"id": "00000000-0000-4000-8000-000000000001", "name": "Example Co", "slug": "example"},
        "centralPlane": {"projectURL": "https://project.supabase.co", "publishableKey": "publishable"},
        "gatewayURL": "wss://gateway.example.com",
        "agentKey": "a" * 64,
    }


class CompanyHostInstallerTests(unittest.TestCase):
    def write_configuration(self, directory, document):
        path = Path(directory) / "connection.json"
        path.write_text(json.dumps(document))
        return path

    def test_rejects_bad_version_types_addresses_and_key(self):
        base = configuration()
        cases = [
            ("schemaVersion", 2),
            ("schemaVersion", "1"),
            ("schemaVersion", True),
            ("schemaVersion", 1.0),
            ("appURL", "https://company.example.com/#fragment"),
            ("appURL", "https://user:pass@company.example.com"),
            ("centralPlane", {"projectURL": "https://project.supabase.co", "publishableKey": 7}),
            ("gatewayURL", "https://gateway.example.com"),
            ("agentKey", "A" * 64),
            ("agentKey", "a" * 63),
        ]
        for field, value in cases:
            with self.subTest(field=field, value=value):
                document = json.loads(json.dumps(base))
                if field == "centralPlane":
                    document[field] = value
                else:
                    document[field] = value
                with tempfile.TemporaryDirectory() as directory:
                    path = self.write_configuration(directory, document)
                    with self.assertRaises(ValueError):
                        installer.read_configuration(path)

    def test_rejects_malformed_documents_before_using_nested_fields(self):
        documents = [
            "not json",
            {},
            {"schemaVersion": 1},
            {**configuration(), "company": []},
            {**configuration(), "company": {**configuration()["company"], "id": "not-a-uuid"}},
            {**configuration(), "appURL": "https://company.example.com\nmalicious"},
        ]
        for document in documents:
            with self.subTest(document=document):
                with tempfile.TemporaryDirectory() as directory:
                    path = Path(directory) / "connection.json"
                    if isinstance(document, str):
                        path.write_text(document)
                    else:
                        path.write_text(json.dumps(document))
                    with self.assertRaises((ValueError, json.JSONDecodeError)):
                        installer.read_configuration(path)

    def test_reinstall_keeps_existing_seeds_and_private_modes(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory) / "state"
            state.mkdir()
            state.chmod(0o755)
            secrets = state / "secrets"
            secrets.mkdir()
            secrets.chmod(0o755)
            seed = secrets / "buzz-key-seed"
            seed.write_text("seed-value\n")
            seed.chmod(0o644)
            agent = secrets / "agent-key"
            agent.write_text("old-agent\n")
            installer.prepare_directory(state, configuration())
            self.assertEqual(seed.read_text(), "seed-value\n")
            self.assertEqual(stat.S_IMODE(seed.stat().st_mode), 0o644)
            self.assertEqual(stat.S_IMODE(state.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE(secrets.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE((state / "connection.json").stat().st_mode), 0o600)
            self.assertEqual(stat.S_IMODE(agent.stat().st_mode), 0o600)

            installer.prepare_directory(state, configuration())
            self.assertEqual(seed.read_text(), "seed-value\n")
            self.assertEqual(agent.read_text(), "a" * 64 + "\n")

    def test_generated_secrets_and_environment_stay_stable_across_rerun(self):
        identity = {"relayOwnerPublicKey": "b" * 64, "agentPrivateKey": "c" * 64}
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory) / "state"
            state.mkdir()
            installer.prepare_directory(state, configuration())
            first = installer.installation_environment(configuration(), state, "host:one", "buzz:one", identity)
            saved = {
                name: (state / "secrets" / name).read_bytes()
                for name in ("postgres-password", "buzz-relay-key", "media-access-key", "media-secret-key")
            }
            second = installer.installation_environment(configuration(), state, "host:one", "buzz:one", identity)
            self.assertEqual(first, second)
            for name, content in saved.items():
                self.assertEqual((state / "secrets" / name).read_bytes(), content)
                self.assertEqual(stat.S_IMODE((state / "secrets" / name).stat().st_mode), 0o600)

    def test_refuses_state_directory_owned_by_another_company_before_mutation(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory) / "state"
            state.mkdir()
            original = configuration()
            original["company"]["id"] = "00000000-0000-4000-8000-000000000009"
            connection = state / "connection.json"
            connection.write_text(json.dumps(original))
            before = connection.read_bytes()

            with self.assertRaisesRegex(ValueError, "another company"):
                installer.prepare_directory(state, configuration())
            self.assertEqual(connection.read_bytes(), before)
            self.assertFalse((state / "secrets").exists())

    def test_compose_environment_quotes_dollar_and_apostrophe_values(self):
        rendered = installer.compose_environment_text({"DOLLAR": "value$HOME", "APOSTROPHE": "Kim's key"})
        self.assertEqual(rendered, "DOLLAR='value$HOME'\nAPOSTROPHE='Kim\\'s key'\n")


if __name__ == "__main__":
    unittest.main()
