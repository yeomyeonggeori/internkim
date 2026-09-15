import importlib.machinery
import importlib.util
import io
from contextlib import redirect_stdout
from pathlib import Path
import subprocess
import tempfile
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


class ShippableTargetTests(unittest.TestCase):
    def create_repository(self):
        directory = tempfile.TemporaryDirectory()
        repository = Path(directory.name)
        self.run_git(repository, "init", "-q")
        self.run_git(repository, "config", "user.email", "test@example.com")
        self.run_git(repository, "config", "user.name", "Test User")
        (repository / "state.txt").write_text("base\n")
        self.run_git(repository, "add", "state.txt")
        self.run_git(repository, "commit", "-qm", "base")
        base_revision = self.run_git(repository, "rev-parse", "HEAD")
        (repository / "state.txt").write_text("main\n")
        self.run_git(repository, "commit", "-qam", "main change")
        main_revision = self.run_git(repository, "rev-parse", "HEAD")
        self.run_git(repository, "branch", "-M", "main")
        self.run_git(repository, "update-ref", "refs/remotes/origin/main", main_revision)
        return directory, repository, base_revision, main_revision

    def run_git(self, repository, *arguments):
        result = subprocess.run(["git", *arguments], cwd=repository, capture_output=True, text=True, check=True)
        return result.stdout.strip()

    def require_target(self, module, repository, revision):
        module.REPOSITORY_ROOT = repository
        module.device_revision_of = lambda target: revision
        return module.require_targets_are_shippable([{"id": "sample", "adminURL": "", "revision": revision}])

    def test_rewritten_revision_with_matching_ancestor_tree_is_shippable(self):
        module = deploy_main_module()
        directory, repository, base_revision, _ = self.create_repository()
        self.addCleanup(directory.cleanup)
        tree = self.run_git(repository, "rev-parse", f"{base_revision}^{{tree}}")
        rewritten_revision = subprocess.run(
            ["git", "commit-tree", tree, "-p", base_revision, "-m", "rewritten"],
            cwd=repository,
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
        output = io.StringIO()
        with redirect_stdout(output):
            self.require_target(module, repository, rewritten_revision)
        self.assertIn(f"deployed {rewritten_revision} has the identical Git tree as main ancestor {base_revision}", output.getvalue())

    def test_different_side_branch_tree_is_refused(self):
        module = deploy_main_module()
        directory, repository, _, _ = self.create_repository()
        self.addCleanup(directory.cleanup)
        self.run_git(repository, "checkout", "-qb", "side")
        (repository / "side.txt").write_text("side\n")
        self.run_git(repository, "add", "side.txt")
        self.run_git(repository, "commit", "-qm", "side change")
        side_revision = self.run_git(repository, "rev-parse", "HEAD")
        with self.assertRaisesRegex(module.Refusal, "exact Git tree are absent"):
            self.require_target(module, repository, side_revision)

    def test_unavailable_revision_is_refused(self):
        module = deploy_main_module()
        directory, repository, _, _ = self.create_repository()
        self.addCleanup(directory.cleanup)
        with self.assertRaisesRegex(module.Refusal, "exact Git tree are absent"):
            self.require_target(module, repository, "unavailable-revision")

    def test_direct_ancestor_revision_is_shippable(self):
        module = deploy_main_module()
        directory, repository, base_revision, _ = self.create_repository()
        self.addCleanup(directory.cleanup)
        self.require_target(module, repository, base_revision)


if __name__ == "__main__":
    unittest.main()
