import fcntl
import importlib.machinery
import importlib.util
import io
import json
import os
import re
import subprocess
import sys
import tempfile
import threading
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


def a_repository_with_a_branch(directory):
    def git(*arguments):
        subprocess.run(["git", *arguments], cwd=directory, check=True, capture_output=True)

    git("init", "-q", "-b", "main")
    git("config", "user.email", "verify@example.com")
    git("config", "user.name", "verify")
    (directory / "docs").mkdir()
    (directory / "docs" / "moved.mdx").write_text("moved")
    (directory / "docs" / "kept.mdx").write_text("kept")
    git("add", ".")
    git("commit", "-qm", "base")
    git("checkout", "-q", "-b", "branch")
    return git


RELATIVE_REFERENCE = re.compile(r"['\"`]((?:\.\./)+[^'\"`$]*)['\"`]")
WEB_SUITE_SOURCES = ["web/src", "web/tests/unit", "web/tests/api", "web/tests/conventions", "web/tests/support"]


def web_suite_references_outside_web(repository_root):
    submodules = submodule_paths(repository_root)
    references = set()
    for source_directory in WEB_SUITE_SOURCES:
        for source in (repository_root / source_directory).rglob("*"):
            if source.suffix not in {".ts", ".svelte", ".js"} or "node_modules" in source.parts:
                continue
            for literal in RELATIVE_REFERENCE.findall(source.read_text(errors="ignore")):
                path = Path(os.path.normpath(source.parent / literal)).relative_to(repository_root, walk_up=True).as_posix()
                if path.startswith(("web/", "..")) or path in {".", "web"}:
                    continue
                references.add(next((submodule for submodule in submodules if path.startswith(submodule + "/")), as_a_repository_path(repository_root, path)))
    return references


def as_a_repository_path(repository_root, path):
    if (repository_root / path).is_dir():
        return f"{path}/"
    if (repository_root / path).exists():
        return path
    return next((f"{path}{suffix}" for suffix in [".ts", ".js", ".svelte"] if (repository_root / f"{path}{suffix}").exists()), path)


def submodule_paths(repository_root):
    listing = subprocess.run(["git", "config", "-f", ".gitmodules", "--get-regexp", "path"], cwd=repository_root, capture_output=True, text=True)
    return [line.split(" ", 1)[1] for line in listing.stdout.splitlines()]


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

    def test_a_relay_change_runs_the_web_tests_that_import_the_relay(self):
        groups = verification_module().GROUPS
        for changed_path in ["host/relay/arrived.ts", "host/relay/acp-session.ts", "host/relay/bun.lock"]:
            selected = {group.name for group in groups if group.is_touched_by([changed_path])}
            self.assertIn("web", selected, changed_path)

    def test_every_file_the_web_suite_reaches_outside_web_selects_the_web_group(self):
        module = verification_module()
        web = next(group for group in module.GROUPS if group.name == "web")
        missed = sorted(path for path in web_suite_references_outside_web(module.REPOSITORY_ROOT) if not web.matches(path) and not web.matches(f"{path}any-file"))
        self.assertEqual(missed, [])

    def test_a_script_line_in_the_package_reaches_no_browser(self):
        groups = verification_module().GROUPS
        selected = {group.name for group in groups if group.is_touched_by(["web/package.json"])}
        self.assertNotIn("e2e", selected)
        selected = {group.name for group in groups if group.is_touched_by(["web/bun.lock"])}
        self.assertIn("e2e", selected)

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


    def test_a_base_that_cannot_be_compared_stops_the_run_instead_of_selecting_nothing(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "REPOSITORY_ROOT", Path(directory)):
            git = a_repository_with_a_branch(Path(directory))
            (Path(directory) / "docs" / "kept.mdx").write_text("changed")
            git("commit", "-qam", "change")
            with self.assertRaises(SystemExit) as stopped:
                module.changed_paths_against("origin/main")
        self.assertIn("cannot compare HEAD with origin/main", str(stopped.exception))

    def test_every_changed_path_is_read_whole_on_both_sides_of_a_rename(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "REPOSITORY_ROOT", Path(directory)):
            root = Path(directory)
            git = a_repository_with_a_branch(root)
            (root / "web").mkdir()
            git("mv", "docs/moved.mdx", "web/moved.mdx")
            git("commit", "-qm", "move")
            (root / "web" / "a name with spaces.ts").write_text("untracked")
            (root / "docs" / "kept.mdx").write_text("uncommitted")
            git("mv", "docs/kept.mdx", "web/kept.mdx")
            changed = module.changed_paths_against("main")
        self.assertEqual(
            changed,
            ["docs/kept.mdx", "docs/moved.mdx", "web/a name with spaces.ts", "web/kept.mdx", "web/moved.mdx"],
        )

    def test_a_group_that_fills_the_machine_waits_while_another_run_holds_every_slot(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "MACHINE_SLOT_DIRECTORY", Path(directory)), patch.object(module, "MACHINE_SLOTS", 1):
            module.verification_run_directory = Path(directory) / "logs"
            slot_held_elsewhere = (Path(directory) / "internkim-verify-slot-0.lock").open("a")
            fcntl.flock(slot_held_elsewhere, fcntl.LOCK_EX)
            heavy = module.Group("heavy", [], [[sys.executable, "-c", "pass"]], fills_the_machine=True)
            light = module.Group("light", [], [[sys.executable, "-c", "pass"]])
            result = []
            waiting = threading.Thread(target=lambda: result.append(module.run_group(heavy)))
            with redirect_stdout(io.StringIO()):
                waiting.start()
                self.assertTrue(module.run_group(light)[1])
                waiting.join(timeout=1)
                self.assertTrue(waiting.is_alive())
                slot_held_elsewhere.close()
                waiting.join(timeout=10)
            self.assertFalse(waiting.is_alive())
            self.assertTrue(result[0][1])

    def test_a_command_waiting_for_the_local_plane_holds_no_machine_slot(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "MACHINE_SLOT_DIRECTORY", Path(directory)), patch.object(module, "MACHINE_SLOTS", 1):
            root = Path(directory)
            module.verification_run_directory = root / "logs"
            observed = root / "observed"
            plane = root / "with-local-plane"
            slot = root / "internkim-verify-slot-0.lock"
            plane.write_text(
                "#!/bin/sh\n"
                f"{sys.executable} -c \"import fcntl; handle = open({str(slot)!r}, 'a'); fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)\" "
                f"&& echo free > {observed} || echo held > {observed}\n"
                "exec \"$@\"\n"
            )
            plane.chmod(0o755)
            with patch.object(module, "WITH_LOCAL_PLANE", str(plane)):
                group = module.Group("heavy", [], [module.holding_the_local_plane([sys.executable, "-c", "pass"])], fills_the_machine=True)
                with redirect_stdout(io.StringIO()):
                    self.assertTrue(module.run_group(group)[1])
            self.assertEqual(observed.read_text().strip(), "free")

    def test_a_group_that_fills_the_machine_takes_its_slot_inside_the_local_plane(self):
        module = verification_module()
        holder = module.machine_slot_holder("heavy")
        command = [sys.executable, "-c", "pass"]
        heavy = module.Group("heavy", [], [], fills_the_machine=True)
        light = module.Group("light", [], [])
        self.assertEqual(heavy.command_to_run(command), [*holder, *command])
        self.assertEqual(heavy.command_to_run(module.holding_the_local_plane(command)), [module.WITH_LOCAL_PLANE, *holder, *module.holding_the_local_plane(command)[1:]])
        self.assertEqual(light.command_to_run(module.holding_the_local_plane(command)), module.holding_the_local_plane(command))
        end_to_end = next(group for group in module.GROUPS if group.name == "e2e")
        with patch.object(type(end_to_end), "needs_the_local_plane", return_value=True):
            self.assertEqual(end_to_end.command_to_run(["scripts/e2e.sh"]), [module.WITH_LOCAL_PLANE, *module.machine_slot_holder("e2e"), "scripts/e2e.sh"])

    def test_a_command_waiting_for_a_machine_slot_says_so_on_the_console(self):
        module = verification_module()
        reading, writing = os.pipe()
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "MACHINE_SLOT_DIRECTORY", Path(directory)), patch.object(module, "MACHINE_SLOTS", 1), patch.object(module, "console_descriptor", return_value=writing):
            module.verification_run_directory = Path(directory) / "logs"
            slot_held_elsewhere = (Path(directory) / "internkim-verify-slot-0.lock").open("a")
            fcntl.flock(slot_held_elsewhere, fcntl.LOCK_EX)
            heavy = module.Group("heavy", [], [[sys.executable, "-c", "pass"]], fills_the_machine=True)
            waiting = threading.Thread(target=lambda: module.run_group(heavy))
            with redirect_stdout(io.StringIO()):
                waiting.start()
                announced = os.read(reading, 200).decode()
                slot_held_elsewhere.close()
                waiting.join(timeout=10)
        os.close(reading)
        os.close(writing)
        self.assertEqual(announced, "wait heavy for one of 1 machine slots\n")

    def test_a_slot_holder_refuses_a_command_line_without_its_separator(self):
        script = Path(__file__).resolve().parents[1] / "verify"
        refused = subprocess.run([sys.executable, str(script), "--hold-a-machine-slot", "/tmp", "1", "1", "heavy", "true"], capture_output=True, text=True)
        self.assertNotEqual(refused.returncode, 0)
        self.assertIn("--", refused.stderr)

    def test_a_group_that_passed_runs_again_only_when_what_selects_it_changes(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "REPOSITORY_ROOT", Path(directory)), patch.object(module, "PASSED_GROUP_DIRECTORY", Path(directory) / "passed"), patch.object(module, "reusing_passes", True):
            root = Path(directory)
            a_repository_with_a_branch(root)
            module.verification_run_directory = root / "logs"
            counter = root / "runs"
            group = module.Group("docs", ["docs/*"], [[sys.executable, "-c", f"import pathlib; path = pathlib.Path({str(counter)!r}); path.write_text(path.read_text() + 'x' if path.exists() else 'x')"]])

            def run():
                module.repository_entries.cache_clear()
                with redirect_stdout(io.StringIO()):
                    return module.run_group(group)

            self.assertTrue(run()[1])
            self.assertIsNone(run()[2])
            (root / "elsewhere.txt").write_text("outside the patterns")
            self.assertIsNone(run()[2])
            (root / "docs" / "kept.mdx").write_text("changed")
            self.assertIsNotNone(run()[2])
            self.assertEqual(counter.read_text(), "xx")

    def test_a_failure_is_never_remembered(self):
        module = verification_module()
        with tempfile.TemporaryDirectory() as directory, patch.object(module, "REPOSITORY_ROOT", Path(directory)), patch.object(module, "PASSED_GROUP_DIRECTORY", Path(directory) / "passed"), patch.object(module, "reusing_passes", True):
            root = Path(directory)
            a_repository_with_a_branch(root)
            module.verification_run_directory = root / "logs"
            group = module.Group("docs", ["docs/*"], [[sys.executable, "-c", "raise SystemExit(1)"]])
            for _ in range(2):
                module.repository_entries.cache_clear()
                with redirect_stdout(io.StringIO()):
                    result = module.run_group(group)
                self.assertFalse(result[1])
                self.assertIsNotNone(result[2])


if __name__ == "__main__":
    unittest.main()
