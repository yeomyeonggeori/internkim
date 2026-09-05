import importlib.machinery
import importlib.util
from pathlib import Path
import unittest


def verification_module():
    script = Path(__file__).resolve().parents[1] / "verify"
    loader = importlib.machinery.SourceFileLoader("verification_plan", str(script))
    specification = importlib.util.spec_from_loader(loader.name, loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


class VerificationPlanTests(unittest.TestCase):
    def test_web_installers_and_consumers_share_a_serial_lane(self):
        groups = verification_module().GROUPS
        web_groups = [group for group in groups if group.working_directory == "web"]
        generator = next(group for group in groups if group.name == "generated-protocol")
        self.assertEqual({group.lane for group in [generator, *web_groups]}, {"web"})

    def test_rubric_changes_verify_every_generated_consumer(self):
        groups = verification_module().GROUPS
        selected = {group.name for group in groups if group.is_touched_by(["internal/tasksize/definitions.json"])}
        self.assertTrue({"go", "web", "generated-protocol"}.issubset(selected))


if __name__ == "__main__":
    unittest.main()
