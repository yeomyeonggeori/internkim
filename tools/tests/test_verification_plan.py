import importlib.machinery
import importlib.util
import io
import json
import re
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
    def test_a_change_inside_one_area_runs_only_that_area_end_to_end(self):
        module = verification_module()
        group = next(group for group in module.GROUPS if group.name == "e2e")
        changed = ["web/src/routes/crm/crm-view-currency.svelte.ts", "web/src/lib/task/task-board.ts", "web/tests/e2e/crm/crm-central.spec.ts"]
        with patch.object(module, "end_to_end_areas", return_value=["crm", "shell", "task"]):
            self.assertTrue(group.is_touched_by(changed))
            self.assertEqual(group.narrowed_to(changed).commands, [["scripts/e2e.sh", "tests/e2e/crm", "tests/e2e/task"]])

    def test_a_shared_change_runs_every_area_end_to_end(self):
        module = verification_module()
        group = next(group for group in module.GROUPS if group.name == "e2e")
        with patch.object(module, "end_to_end_areas", return_value=["crm", "shell", "task"]):
            for shared in ["web/src/app.css", "web/src/lib/components/ui/button/button.svelte", "web/src/routes/+layout.svelte", "web/tests/e2e/support/central-test-utils.ts", "web/src/lib/server/public-api/record/company-tools.ts"]:
                self.assertEqual(group.narrowed_to(["web/src/routes/crm/text.ts", shared]).commands, [["scripts/e2e.sh"]], shared)

    def test_a_database_change_runs_only_the_specs_on_the_local_plane(self):
        module = verification_module()
        group = next(group for group in module.GROUPS if group.name == "e2e")
        with patch.object(module, "end_to_end_areas", return_value=["crm", "shell", "task"]):
            for database in ["supabase/migrations/20990101000000_example.sql", "supabase/seed.dev.sql"]:
                self.assertTrue(group.is_touched_by([database]), database)
                self.assertEqual(group.narrowed_to([database]).commands, [["scripts/e2e.sh", "--project=central"]], database)

    def test_a_database_change_beside_one_area_runs_that_area_on_the_other_planes(self):
        module = verification_module()
        group = next(group for group in module.GROUPS if group.name == "e2e")
        changed = ["supabase/migrations/20990101000000_example.sql", "web/src/routes/crm/text.ts"]
        with patch.object(module, "end_to_end_areas", return_value=["crm", "shell", "task"]):
            self.assertEqual(group.narrowed_to(changed).commands, [
                ["scripts/e2e.sh", "--project=central"],
                ["scripts/e2e.sh", "--project=fixture", "--project=mocked", "--project=standalone", "tests/e2e/crm"],
            ])

    def test_every_spec_sits_in_an_area_named_after_a_route_or_the_shell(self):
        module = verification_module()
        routes = {path.name for path in (module.REPOSITORY_ROOT / "web" / "src" / "routes").iterdir() if path.is_dir()}
        self.assertEqual(sorted(module.END_TO_END_DIRECTORY.glob("*.spec.ts")), [])
        self.assertEqual(set(module.end_to_end_areas()) - routes, {"shell"})

    def test_every_python_test_under_tools_is_run_by_a_group(self):
        module = verification_module()
        commands = [" ".join(command) for group in module.GROUPS for command in group.commands]
        for test_file in sorted((module.REPOSITORY_ROOT / "tools" / "tests").glob("test_*.py")):
            self.assertTrue(any(test_file.name in command for command in commands), f"no group runs tools/tests/{test_file.name}")

    def test_every_web_test_directory_is_run_by_a_group_or_named_as_run_elsewhere(self):
        module = verification_module()
        run_elsewhere = {
            "plane": "./internkim dev plane runs it in a disposable Linux VM",
            "pilot": "a model pilot harness run by `bun run pilot`, not a test suite",
            "support": "helpers the suites import",
            "fixtures": "data the suites read",
        }
        commands = [" ".join(command) for group in module.GROUPS for command in group.commands]
        scripts = json.loads((module.REPOSITORY_ROOT / "web" / "package.json").read_text())["scripts"]
        named_scripts = {name for command in commands for name in re.findall(r"bun run ([\w:-]+)", command)}
        expanded = commands + [scripts[name] for name in named_scripts if name in scripts]
        for directory in sorted(path.name for path in (module.REPOSITORY_ROOT / "web" / "tests").iterdir() if path.is_dir()):
            if directory in run_elsewhere or directory == "e2e":
                continue
            self.assertTrue(any(f"tests/{directory}" in command for command in expanded), f"no group runs web/tests/{directory}")

    def test_web_installers_and_consumers_share_a_serial_lane(self):
        groups = verification_module().GROUPS
        web_groups = [group for group in groups if group.working_directory == "web"]
        generator = next(group for group in groups if group.name == "generated-protocol")
        self.assertEqual({group.lane for group in [generator, *web_groups]}, {"web"})

    def test_android_changes_run_the_android_group(self):
        groups = verification_module().GROUPS
        for changed_path in [
            "web/attendance-widget/android/src/main/java/app/intern/kim/attendance/AttendanceWidgetStore.java",
            "web/android/app/src/main/java/app/intern/kim/attendance/AttendanceWidgetPlugin.java",
            "web/sender-notification/android/src/main/java/app/intern/kim/notification/SentNotification.java",
        ]:
            selected = {group.name for group in groups if group.is_touched_by([changed_path])}
            self.assertIn("android", selected)

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
                [".dependency/internkim-plugin/tests/run"],
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
