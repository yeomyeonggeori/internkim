import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

TOOL_PATH = Path(__file__).resolve().parent.parent / "verify-office-host-contract"
specification = importlib.util.spec_from_loader("verify_office_host_contract", loader=None)
verify_office_host_contract = importlib.util.module_from_spec(specification)
verify_office_host_contract.__file__ = str(TOOL_PATH)
exec(compile(TOOL_PATH.read_text(), str(TOOL_PATH), "exec"), verify_office_host_contract.__dict__)

CONTRACT = {"runtimeContextVariable": "OFFICE_RUNTIME_CONTEXT", "sourceSuffix": ".source.json", "deliverableExtensions": [".docx", ".pdf"]}


class VerifyOfficeHostContractTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        for copy in (verify_office_host_contract.PLUGIN_CONTRACT, verify_office_host_contract.HOST_CONTRACT):
            (self.root / copy).parent.mkdir(parents=True, exist_ok=True)
            (self.root / copy).write_text(json.dumps(CONTRACT, indent=2))

    def tearDown(self):
        self.temporary.cleanup()

    def test_two_copies_with_the_same_content_agree_however_they_are_formatted(self):
        (self.root / verify_office_host_contract.HOST_CONTRACT).write_text(json.dumps(CONTRACT))
        self.assertEqual(verify_office_host_contract.findings_for(self.root), [])

    def test_a_field_the_host_copy_lacks_is_named(self):
        drifted = {key: value for key, value in CONTRACT.items() if key != "sourceSuffix"}
        (self.root / verify_office_host_contract.HOST_CONTRACT).write_text(json.dumps(drifted))
        findings = verify_office_host_contract.findings_for(self.root)
        self.assertEqual(len(findings), 1)
        self.assertIn("sourceSuffix", findings[0])

    def test_a_missing_copy_is_a_finding(self):
        (self.root / verify_office_host_contract.PLUGIN_CONTRACT).unlink()
        self.assertIn(str(verify_office_host_contract.PLUGIN_CONTRACT), verify_office_host_contract.findings_for(self.root)[0])


if __name__ == "__main__":
    unittest.main()
