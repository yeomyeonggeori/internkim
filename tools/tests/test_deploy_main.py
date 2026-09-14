import importlib.machinery
import importlib.util
import io
from contextlib import redirect_stdout
from pathlib import Path
import unittest


def deploy_main_module():
    script = Path(__file__).resolve().parents[1] / "deploy-main"
    loader = importlib.machinery.SourceFileLoader("deploy_main", str(script))
    specification = importlib.util.spec_from_loader(loader.name, loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


class FakeCompletedProcess:
    def __init__(self, returncode):
        self.returncode = returncode


class AncestorDecisionTests(unittest.TestCase):
    def test_ancestor_revision_is_shippable(self):
        module = deploy_main_module()
        self.assertTrue(module.is_ancestor("abc123", "origin/main", runner=lambda command: FakeCompletedProcess(0)))

    def test_non_ancestor_revision_is_not_shippable(self):
        module = deploy_main_module()
        self.assertFalse(module.is_ancestor("abc123", "origin/main", runner=lambda command: FakeCompletedProcess(1)))

    def test_ancestor_check_runs_merge_base_is_ancestor(self):
        module = deploy_main_module()
        seen_commands = []

        def recording_runner(command):
            seen_commands.append(command)
            return FakeCompletedProcess(0)

        module.is_ancestor("deadbeef", "origin/main", runner=recording_runner)
        self.assertEqual(seen_commands, [["git", "merge-base", "--is-ancestor", "deadbeef", "origin/main"]])


class AvailableComponentsParserTests(unittest.TestCase):
    def test_parses_the_available_line(self):
        module = deploy_main_module()
        help_text = (
            "Usage: internkim deploy [OPTIONS]\n\n"
            "Options:\n"
            "  --components <list>  Comma-separated component names to include.\n"
            "                       Available: admind, blueclawPayload, capabilityd, skills, web\n"
            "                       Example: --components admind,web\n"
        )
        self.assertEqual(
            module.parse_available_components(help_text),
            ["admind", "blueclawPayload", "capabilityd", "skills", "web"],
        )

    def test_refuses_when_no_available_line_is_printed(self):
        module = deploy_main_module()
        with self.assertRaises(module.Refusal):
            module.parse_available_components("Usage: internkim deploy [OPTIONS]\n")

    def test_ignores_surrounding_whitespace(self):
        module = deploy_main_module()
        help_text = "                       Available:   admind ,  web  \n"
        self.assertEqual(module.parse_available_components(help_text), ["admind", "web"])


class CleanTreeCheckTests(unittest.TestCase):
    def test_clean_when_no_lines_are_dirty(self):
        module = deploy_main_module()
        self.assertTrue(module.working_tree_is_clean([]))

    def test_dirty_on_an_unrelated_change(self):
        module = deploy_main_module()
        self.assertFalse(module.working_tree_is_clean(["?? tools/deploy-main"]))

    def test_ignores_the_stray_blueclaw_test_artifact(self):
        module = deploy_main_module()
        status_lines = ["?? .dependency/blueclaw/internal/app/.blueclaw/"]
        self.assertTrue(module.working_tree_is_clean(status_lines))

    def test_stray_artifact_does_not_mask_other_dirt(self):
        module = deploy_main_module()
        status_lines = [
            "?? .dependency/blueclaw/internal/app/.blueclaw/",
            " M .dependency/blueclaw",
        ]
        self.assertFalse(module.working_tree_is_clean(status_lines))


class PlanTests(unittest.TestCase):
    def test_plan_prints_every_step_and_runs_nothing(self):
        module = deploy_main_module()
        output = io.StringIO()
        with redirect_stdout(output):
            module.print_plan()
        printed = output.getvalue()
        for step in module.PLAN_STEPS:
            self.assertIn(step, printed)


if __name__ == "__main__":
    unittest.main()
