import importlib.machinery
import importlib.util
import io
import sys
import tempfile
import time
from pathlib import Path
from contextlib import redirect_stdout
from unittest.mock import patch
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

    def test_relay_message_tag_changes_verify_the_shell_that_matches_it(self):
        groups = verification_module().GROUPS
        selected = {group.name for group in groups if group.is_touched_by(["host/relay/arrived.ts"])}
        self.assertIn("web", selected)

    def test_internkim_plugin_changes_select_the_plugin_group(self):
        groups = verification_module().GROUPS
        plugin_group = next(group for group in groups if group.name == "internkim-plugin")
        for changed_path in [".dependency/internkim-plugin", ".dependency/internkim-plugin/mcp.json", "tools/verify-agent-plugin"]:
            selected = {group.name for group in groups if group.is_touched_by([changed_path])}
            self.assertIn("internkim-plugin", selected)
        self.assertEqual(plugin_group.working_directory, ".")
        self.assertEqual(
            plugin_group.commands,
            [
                ["python3", "-m", "unittest", "discover", "-s", "tools/tests", "-p", "test_verify_agent_plugin.py"],
                ["tools/verify-agent-plugin"],
                ["python3", ".dependency/internkim-plugin/kim.intern/tests/test_data_room.py"],
            ],
        )

    def test_plan_prints_groups_and_commands_without_running(self):
        module = verification_module()
        output = io.StringIO()
        with redirect_stdout(output):
            module.print_plan([module.GROUPS[0]])
        self.assertIn("verification-plan", output.getvalue())
        self.assertIn("python3 -m unittest", output.getvalue())

    def test_main_plan_does_not_start_subprocesses(self):
        module = verification_module()
        with patch.object(module.subprocess, "run", side_effect=AssertionError("plan executed a subprocess")), patch.object(module.subprocess, "Popen", side_effect=AssertionError("plan executed a subprocess")), patch.object(sys, "argv", ["verify", "--only", "verification-plan", "--plan"]):
            self.assertEqual(module.main(), 0)

    def test_run_group_flushes_output_before_child_exit(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory:
            module.verification_run_directory = Path(directory)
            ready_path = Path(directory) / "ready"
            release_path = Path(directory) / "release"
            child_code = "import pathlib,sys,time; ready=pathlib.Path(sys.argv[1]); release=pathlib.Path(sys.argv[2]); print('marker', flush=True); ready.write_text('ready');\nwhile not release.exists(): time.sleep(.01)"
            group = module.Group("live", [], [[sys.executable, "-c", child_code, str(ready_path), str(release_path)]])
            import threading
            result = []
            thread = threading.Thread(target=lambda: result.append(module.run_group(group)))
            thread.start()
            log_path = Path(directory) / "live" / "1.log"
            for _ in range(20):
                if ready_path.exists() and log_path.exists() and "marker" in log_path.read_text():
                    break
                time.sleep(.02)
            self.assertTrue(ready_path.exists())
            self.assertIn("marker", log_path.read_text())
            self.assertFalse(release_path.exists())
            release_path.write_text("release")
            thread.join()
            self.assertTrue(result[0][1])

    def test_lane_runs_after_failure_and_separate_runs_keep_logs(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory:
            module.verification_run_directory = Path(directory) / "one"
            groups = [module.Group("first", [], [[sys.executable, "-c", "print('failed'); raise SystemExit(2)"]], lane="serial"), module.Group("second", [], [[sys.executable, "-c", "print('continued')"]], lane="serial")]
            results = module.run_lane(groups)
            self.assertFalse(results[0][1])
            self.assertTrue(results[1][1])
            module.verification_run_directory = Path(directory) / "two"
            module.run_group(groups[1])
            self.assertTrue((Path(directory) / "one" / "second" / "1.log").exists())
            self.assertTrue((Path(directory) / "two" / "second" / "1.log").exists())


if __name__ == "__main__":
    unittest.main()
