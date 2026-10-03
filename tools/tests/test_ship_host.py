import contextlib
import hashlib
import importlib.machinery
import importlib.util
import io
import tempfile
import unittest
from pathlib import Path

repository_root = Path(__file__).resolve().parents[2]

NEW_TAG = "v2026.10.03.045216"
PREVIOUS_TAG = "v2026.10.02.172114"
HEALTHY_UNITS = "internkim-admind.service active\nblueclaw.service active\nchatd.service active\nbuzz-relay.service active\n"


def load_script():
    loader = importlib.machinery.SourceFileLoader("ship_host_script", str(repository_root / "tools" / "ship-host"))
    specification = importlib.util.spec_from_loader("ship_host_script", loader)
    module = importlib.util.module_from_spec(specification)
    loader.exec_module(module)
    return module


ship_host = load_script()


class ScriptedCommands:
    def __init__(self, host_answers=None, failing=()):
        self.calls = []
        self.host_answers = {
            "dpkg --print-architecture": "arm64\n",
            "dpkg-query": NEW_TAG.lstrip("v") + "\n",
            "systemctl list-unit-files": HEALTHY_UNITS,
            "systemctl show -p NRestarts": "0\n",
            **(host_answers or {}),
        }
        self.failing = failing

    def __call__(self, arguments):
        self.calls.append(arguments)
        for marker in self.failing:
            if marker in " ".join(arguments):
                raise ship_host.CommandFailure(f"{marker} failed")
        return self.answer(arguments)

    def answer(self, arguments):
        line = " ".join(arguments)
        if arguments[:3] == ["gh", "release", "view"]:
            return PREVIOUS_TAG + "\n"
        if arguments[:2] == ["git", "show"]:
            return "1791003136\n"
        if arguments[:2] == ["git", "rev-parse"]:
            return "0123456789abcdef\n"
        if "--channel testing" in line:
            return f"released {NEW_TAG} on testing: https://example.com\n"
        if arguments[:3] == ["gh", "release", "download"]:
            return self.write_download(arguments)
        if "ssh" in arguments:
            return self.host_answer(arguments[-1])
        return ""

    def write_download(self, arguments):
        directory = Path(arguments[arguments.index("--dir") + 1])
        directory.mkdir(parents=True, exist_ok=True)
        package = directory / ship_host.package_asset_name()
        package.write_bytes(b"package")
        digest = hashlib.sha256(b"package").hexdigest()
        (directory / ship_host.CHECKSUMS_NAME).write_text(f"{digest}  {package.name}\n")
        return ""

    def host_answer(self, script):
        for marker, answer in self.host_answers.items():
            if marker in script:
                return "SSH: operator@host\n" + answer
        return "SSH: operator@host\n"

    def matching(self, text):
        return [index for index, call in enumerate(self.calls) if text in " ".join(call)]


def shipment_with(commands, options=None, directory="."):
    options = options or ship_host.Options(skip_host=False, is_plan=False)
    return ship_host.Shipment(options=options, run=commands, sleep=lambda seconds: None, directory=Path(directory))


def ship_quietly(shipment):
    output = io.StringIO()
    with contextlib.redirect_stdout(output):
        ship_host.ship(shipment)
    return output.getvalue()


class ArgumentTest(unittest.TestCase):
    def test_nothing_is_the_whole_run(self):
        self.assertEqual(ship_host.parse_arguments([]), ship_host.Options(skip_host=False, is_plan=False))

    def test_skip_host_and_plan(self):
        options = ship_host.parse_arguments(["--skip-host", "--plan"])
        self.assertEqual(options, ship_host.Options(skip_host=True, is_plan=True))

    def test_nothing_else_is_accepted(self):
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            ship_host.parse_arguments(["--channel", "stable"])


class TagTest(unittest.TestCase):
    def test_tag_is_the_commit_time_in_utc(self):
        self.assertEqual(ship_host.tag_for_commit_time("0"), "v1970.01.01.000000")
        self.assertEqual(ship_host.tag_for_commit_time("1791003136"), NEW_TAG)

    def test_tag_is_read_from_what_release_host_prints(self):
        output = f"building\nreleased {NEW_TAG} on testing: https://github.com/example/releases/tag/{NEW_TAG}\n"
        self.assertEqual(ship_host.tag_from_release_output(output), NEW_TAG)

    def test_output_without_a_tag_stops_the_run(self):
        with self.assertRaises(ship_host.ShipFailure):
            ship_host.tag_from_release_output("promoted v2026.10.03.045216 from testing to stable\n")


class PlanTest(unittest.TestCase):
    def test_plan_names_the_tags_and_changes_nothing(self):
        commands = ScriptedCommands()
        options = ship_host.Options(skip_host=False, is_plan=True)
        output = ship_quietly(shipment_with(commands, options))
        self.assertIn(NEW_TAG, output)
        self.assertIn(PREVIOUS_TAG, output)
        self.assertEqual(len([line for line in output.splitlines() if line[:1].isdigit()]), 7)
        self.assertEqual(
            [call[:3] for call in commands.calls], [["gh", "release", "view"], ["git", "show", "-s"]]
        )

    def test_skip_host_plans_no_host_step(self):
        options = ship_host.Options(skip_host=True, is_plan=True)
        output = ship_quietly(shipment_with(ScriptedCommands(), options))
        self.assertNotIn("our own host", output)
        self.assertEqual(len([line for line in output.splitlines() if line[:1].isdigit()]), 6)


class OrderTest(unittest.TestCase):
    def test_previous_stable_is_read_before_anything_is_promoted(self):
        commands = ScriptedCommands()
        with tempfile.TemporaryDirectory() as directory:
            ship_quietly(shipment_with(commands, directory=directory))
        self.assertLess(commands.matching("gh release view")[0], commands.matching("--channel stable")[0])

    def test_steps_run_in_order(self):
        commands = ScriptedCommands()
        with tempfile.TemporaryDirectory() as directory:
            ship_quietly(shipment_with(commands, directory=directory))
        positions = [
            commands.matching("release-tree")[0],
            commands.matching("--channel testing")[0],
            commands.matching("gh release download")[0],
            commands.matching("test-native-install")[0],
            commands.matching("--people-upgrade")[0],
            commands.matching("--channel stable")[0],
            commands.matching("install.sh")[0],
        ]
        self.assertEqual(positions, sorted(positions))

    def test_skip_host_stops_after_promotion(self):
        commands = ScriptedCommands()
        options = ship_host.Options(skip_host=True, is_plan=False)
        with tempfile.TemporaryDirectory() as directory:
            ship_quietly(shipment_with(commands, options, directory))
        self.assertTrue(commands.matching("--channel stable"))
        self.assertFalse(commands.matching("ssh"))

    def test_a_tree_that_is_not_origin_main_cuts_nothing(self):
        commands = ScriptedCommands()
        original = commands.answer
        commands.answer = lambda arguments: "other\n" if arguments[:3] == ["git", "rev-parse", "origin/main"] else original(arguments)
        with self.assertRaises(ship_host.ShipFailure):
            ship_quietly(shipment_with(commands))
        self.assertFalse(commands.matching("--channel testing"))

    def test_a_failed_rig_promotes_nothing_and_touches_no_release(self):
        commands = ScriptedCommands(failing=("--people-upgrade",))
        with tempfile.TemporaryDirectory() as directory, self.assertRaises(ship_host.CommandFailure):
            ship_quietly(shipment_with(commands, directory=directory))
        self.assertFalse(commands.matching("--channel stable"))
        self.assertFalse(commands.matching("release edit"))
        self.assertFalse(commands.matching("ssh"))

    def test_a_package_the_checksums_do_not_vouch_for_stops_the_run(self):
        commands = ScriptedCommands()
        original = commands.write_download

        def corrupting(arguments):
            original(arguments)
            Path(arguments[arguments.index("--dir") + 1], ship_host.package_asset_name()).write_bytes(b"changed")
            return ""

        commands.write_download = corrupting
        with tempfile.TemporaryDirectory() as directory, self.assertRaises(ship_host.ShipFailure):
            ship_quietly(shipment_with(commands, directory=directory))
        self.assertFalse(commands.matching("test-native-install"))


class RollbackTest(unittest.TestCase):
    def failing_host(self, host_answers=None, failing=()):
        commands = ScriptedCommands(host_answers, failing)
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ship_host.ShipFailure) as raised:
                ship_quietly(shipment_with(commands, directory=directory))
        return commands, raised.exception

    def test_a_restarting_agent_rolls_the_host_and_stable_back_in_order(self):
        readings = iter(["0\n", "3\n"])
        commands = ScriptedCommands()
        original = commands.host_answer
        commands.host_answer = lambda script: "SSH: x\n" + next(readings) if "NRestarts" in script else original(script)
        with tempfile.TemporaryDirectory() as directory, self.assertRaises(ship_host.ShipFailure) as raised:
            ship_quietly(shipment_with(commands, directory=directory))
        order = [
            commands.matching("--allow-downgrades")[0],
            commands.matching("release edit " + NEW_TAG + " --repo")[0],
            commands.matching("--channel stable --version " + PREVIOUS_TAG)[0],
            commands.matching("release edit " + PREVIOUS_TAG + " --repo")[0],
        ]
        self.assertEqual(order, sorted(order))
        self.assertIn("restarted 3 times", str(raised.exception))
        self.assertIn("rolled back to " + PREVIOUS_TAG, str(raised.exception))

    def test_the_downgrade_fetches_the_previous_tag_for_the_hosts_architecture(self):
        commands, _ = self.failing_host({"dpkg --print-architecture": "amd64\n", "dpkg-query": "0.0.0\n"})
        downgrade = commands.calls[commands.matching("--allow-downgrades")[0]][-1]
        self.assertIn(f"{PREVIOUS_TAG}/internkim-amd64.deb", downgrade)

    def test_the_failed_tag_is_marked_a_prerelease(self):
        commands, _ = self.failing_host({"dpkg-query": "0.0.0\n"})
        edit = commands.calls[commands.matching("release edit " + NEW_TAG)[0]]
        self.assertIn("--prerelease", edit)

    def test_a_wrong_version_names_what_failed(self):
        _, failure = self.failing_host({"dpkg-query": "2026.10.02.172114\n"})
        self.assertIn("upgrade our own host", str(failure))
        self.assertIn("is 2026.10.02.172114", str(failure))

    def test_a_unit_that_is_not_active_fails_the_upgrade(self):
        _, failure = self.failing_host({"systemctl list-unit-files": HEALTHY_UNITS.replace("chatd.service active", "chatd.service failed")})
        self.assertIn("chatd.service is failed", str(failure))

    def test_every_rollback_action_is_tried_and_the_ones_that_failed_are_named(self):
        commands = ScriptedCommands({"dpkg-query": "0.0.0\n"}, failing=("--allow-downgrades",))
        with tempfile.TemporaryDirectory() as directory, self.assertRaises(ship_host.ShipFailure) as raised:
            ship_quietly(shipment_with(commands, directory=directory))
        self.assertTrue(commands.matching("--prerelease"))
        self.assertIn("rollback left undone", str(raised.exception))
        self.assertIn("roll the host back", str(raised.exception))

    def test_a_host_that_cannot_be_read_is_not_rolled_back(self):
        commands = ScriptedCommands(failing=("--print-architecture",))
        with tempfile.TemporaryDirectory() as directory, self.assertRaises(ship_host.CommandFailure):
            ship_quietly(shipment_with(commands, directory=directory))
        self.assertFalse(commands.matching("release edit"))


class ReportParsingTest(unittest.TestCase):
    def test_inactive_units_are_named(self):
        self.assertEqual(ship_host.inactive_units(HEALTHY_UNITS), [])
        self.assertEqual(ship_host.inactive_units("blueclaw.service activating\n"), ["blueclaw.service is activating"])

    def test_an_empty_report_is_a_failure(self):
        self.assertEqual(ship_host.inactive_units(""), ["no unit was reported"])

    def test_checksums_are_checked_per_asset(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            (directory / "a.deb").write_bytes(b"a")
            (directory / "b.deb").write_bytes(b"b")
            (directory / ship_host.CHECKSUMS_NAME).write_text(
                f"{hashlib.sha256(b'a').hexdigest()}  a.deb\n{hashlib.sha256(b'x').hexdigest()}  b.deb\n"
            )
            self.assertEqual(ship_host.mismatched_assets(directory, ["a.deb", "b.deb"]), ["b.deb"])


if __name__ == "__main__":
    unittest.main()
