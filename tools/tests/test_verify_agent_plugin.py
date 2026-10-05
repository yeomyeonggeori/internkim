import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

TOOL_PATH = Path(__file__).resolve().parent.parent / "verify-agent-plugin"
specification = importlib.util.spec_from_loader("verify_agent_plugin", loader=None)
verify_agent_plugin = importlib.util.module_from_spec(specification)
exec(compile(TOOL_PATH.read_text(), str(TOOL_PATH), "exec"), verify_agent_plugin.__dict__)


def write_conforming_plugin(plugin_root: Path) -> None:
    (plugin_root / "plugin.json").write_text(json.dumps({
        "$schema": verify_agent_plugin.PLUGIN_SCHEMA,
        "name": "sample",
        "version": "1.0.0",
    }))
    (plugin_root / "mcp.json").write_text(json.dumps({
        "$schema": verify_agent_plugin.MCP_SCHEMA,
        "mcpServers": {"sample": {"type": "streamable-http", "url": "https://example.com/mcp"}},
    }))
    skill = plugin_root / "skills" / "greeting"
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text("---\nname: greeting\ndescription: Greets.\n---\n")
    (plugin_root / "README.md").write_text("sample\n")


class VerifyAgentPluginTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.plugin_root = Path(self.temporary.name)
        write_conforming_plugin(self.plugin_root)

    def tearDown(self):
        self.temporary.cleanup()

    def test_a_conforming_plugin_has_no_findings(self):
        self.assertEqual(verify_agent_plugin.findings_for(self.plugin_root), [])

    def test_a_reverse_domain_directory_is_allowed(self):
        (self.plugin_root / "com.example.client").mkdir()
        self.assertEqual(verify_agent_plugin.findings_for(self.plugin_root), [])

    def test_license_and_notice_files_are_allowed(self):
        (self.plugin_root / "LICENSE").write_text("license\n")
        (self.plugin_root / "NOTICE").write_text("adapted material\n")
        self.assertEqual(verify_agent_plugin.findings_for(self.plugin_root), [])

    def test_a_client_manifest_at_the_root_is_a_finding(self):
        (self.plugin_root / ".claude-plugin").mkdir()
        (self.plugin_root / ".mcp.json").write_text("{}")
        findings = verify_agent_plugin.findings_for(self.plugin_root)
        self.assertEqual(len(findings), 2)
        self.assertTrue(all("reverse-domain" in finding for finding in findings))

    def test_marketplace_catalogs_and_tests_are_allowed(self):
        (self.plugin_root / ".claude-plugin").mkdir()
        (self.plugin_root / ".claude-plugin" / "marketplace.json").write_text("{}")
        (self.plugin_root / ".agents" / "plugins").mkdir(parents=True)
        (self.plugin_root / ".agents" / "plugins" / "marketplace.json").write_text("{}")
        (self.plugin_root / "tests").mkdir()
        (self.plugin_root / "tests" / "test_bundle.py").write_text("")
        self.assertEqual(verify_agent_plugin.findings_for(self.plugin_root), [])

    def test_a_client_manifest_beside_a_catalog_is_a_finding(self):
        (self.plugin_root / ".claude-plugin").mkdir()
        (self.plugin_root / ".claude-plugin" / "marketplace.json").write_text("{}")
        (self.plugin_root / ".claude-plugin" / "plugin.json").write_text("{}")
        findings = verify_agent_plugin.findings_for(self.plugin_root)
        self.assertEqual(len(findings), 1)
        self.assertIn("only its marketplace catalog", findings[0])

    def test_a_package_manifest_at_the_root_is_a_finding(self):
        (self.plugin_root / "package.json").write_text("{}")
        self.assertEqual(len(verify_agent_plugin.findings_for(self.plugin_root)), 1)

    def test_a_field_outside_the_spec_is_a_finding(self):
        manifest = json.loads((self.plugin_root / "plugin.json").read_text())
        manifest["skills"] = ["./skills"]
        (self.plugin_root / "plugin.json").write_text(json.dumps(manifest))
        self.assertEqual(verify_agent_plugin.findings_for(self.plugin_root), ["plugin.json: skills is not a permitted top-level field"])

    def test_an_unknown_transport_is_a_finding(self):
        (self.plugin_root / "mcp.json").write_text(json.dumps({
            "$schema": verify_agent_plugin.MCP_SCHEMA,
            "mcpServers": {"sample": {"type": "http", "url": "https://example.com/mcp"}},
        }))
        findings = verify_agent_plugin.findings_for(self.plugin_root)
        self.assertEqual(len(findings), 1)
        self.assertIn("transport 'http'", findings[0])

    def test_a_skill_without_its_document_is_a_finding(self):
        (self.plugin_root / "skills" / "empty").mkdir()
        self.assertEqual(verify_agent_plugin.findings_for(self.plugin_root), ["skills/empty: has no SKILL.md"])


if __name__ == "__main__":
    unittest.main()
