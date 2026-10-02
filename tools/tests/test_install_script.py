import hashlib
import functools
import http.server
import os
from pathlib import Path
import re
import subprocess
import tempfile
import threading
import unittest


repository_root = Path(__file__).resolve().parents[2]
install_script = repository_root / "web/static/install.sh"
formula_source = repository_root / "internal/runtime/blueclaw/company_host_formula.go"
package_formats_source = repository_root / "internal/cli/release_package.go"
package_targets_source = repository_root / "internal/cli/release_package_payload.go"
github_downloads = "https://github.com/yeomyeonggeori/internkim/releases"
github_newest_release = "https://api.github.com/repos/yeomyeonggeori/internkim/releases?per_page=1"

# `install` is the only command the script uses to put a file where root can
# see it, and its destination is always the last argument. Rewriting that one
# argument under a sandbox is what lets this test read what the script wrote to
# /etc and /usr/share without the test machine having either path touched.
def assigned_value(setting):
    match = re.search(rf'^{setting}="([^"$]+)"$', install_script.read_text(), re.MULTILINE)
    if match is None:
        raise AssertionError(f"install.sh no longer assigns {setting} a literal")
    return match.group(1)


def declared_asset_names():
    """The names `internkim release packages` gives each format and architecture.

    install.sh spells the same names, and the release the tests serve is built
    from these, so a script that asks for another name finds nothing to install.
    """
    suffixes = re.findall(r'^\t\tSuffix:\s+"([^"]+)"', package_formats_source.read_text(), re.MULTILINE)
    architectures = re.findall(r'\{Architecture: "([a-z0-9]+)"', package_targets_source.read_text())
    if len(suffixes) != 3 or len(architectures) != 2:
        raise AssertionError(f"internal/cli no longer declares the formats and targets this reads: {suffixes}, {architectures}")
    return {
        (architecture, suffix): f"internkim-{architecture}{suffix}"
        for architecture in architectures
        for suffix in suffixes
    }


def declared_tap():
    source = formula_source.read_text()
    parts = []
    for name in ("HomebrewTapOwner", "HomebrewTapName"):
        match = re.search(rf'^\t{name}\s*=\s*"([^"]+)"', source, re.MULTILINE)
        if match is None:
            raise AssertionError(f"internal/runtime/blueclaw no longer declares {name}")
        parts.append(match.group(1))
    return "/".join(parts)


install_shim = """#!/usr/bin/env python3
import os
import shutil
import sys

sandbox = os.environ["INTERNKIM_TEST_SANDBOX"]
arguments = sys.argv[1:]
mode = 0o755
makes_a_directory = False
positional = []
index = 0
while index < len(arguments):
    if arguments[index] == "-d":
        makes_a_directory = True
    elif arguments[index] == "-m":
        index += 1
        mode = int(arguments[index], 8)
    else:
        positional.append(arguments[index])
    index += 1

destination = positional[-1]
sandboxed = sandbox + destination if destination.startswith("/") else destination
if makes_a_directory:
    os.makedirs(sandboxed, exist_ok=True)
else:
    shutil.copyfile(positional[0], sandboxed)
os.chmod(sandboxed, mode)
"""


def recording_shim(name, log_path, exit_code=0, output=""):
    return (
        "#!/bin/sh\n"
        f'printf "%s\\n" "{name} $*" >> "{log_path}"\n'
        + (f'echo "{output}"\n' if output else "")
        + f"exit {exit_code}\n"
    )
class QuietRequestHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, format, *arguments):
        pass


class InstallScriptTests(unittest.TestCase):
    def run_install(self, machine, shims=()):
        environment = dict(os.environ)
        directories = [*shims, self.uname_shim(*machine)]
        environment["PATH"] = os.pathsep.join([*directories, self.path_without_a_package_manager()])
        return subprocess.run(
            ["sh", str(install_script), "host"],
            capture_output=True,
            text=True,
            env=environment,
        )

    def path_without_a_package_manager(self):
        """A PATH on which the script can find neither apt-get nor brew.

        `host` reaches for a package manager whenever one is there, so a test
        of the machine without one has to hide them. On this Mac that means
        hiding brew; on a Debian build machine it means hiding apt-get, and a
        test that passed only where it was written proves nothing.
        """
        return os.pathsep.join(
            directory
            for directory in os.environ["PATH"].split(os.pathsep)
            if directory and not any((Path(directory) / name).exists() for name in ("apt-get", "dnf", "pacman", "brew"))
        )

    def uname_shim(self, operating_system, architecture):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        shim = directory / "uname"
        shim.write_text(
            "#!/bin/sh\n"
            'case "$1" in\n'
            f'  -s) echo {operating_system} ;;\n'
            f'  -m) echo {architecture} ;;\n'
            '  *) exit 1 ;;\n'
            "esac\n"
        )
        shim.chmod(0o755)
        return str(directory)

    def test_the_tap_the_script_adds_is_the_one_the_formula_is_published_to(self):
        """`internal/runtime/blueclaw` declares the tap and renders the formula
        that lives in it. A literal here would be a second declaration, and a
        machine that adds a tap the formula was never committed to installs
        nothing."""
        self.assertEqual(assigned_value("homebrew_tap"), declared_tap())

    def test_a_mac_with_homebrew_installs_the_formula_from_the_tap(self):
        """The published one line has to leave a registered Homebrew install, or
        `brew upgrade` and `brew uninstall` have nothing to act on afterwards."""
        log_path = Path(self.enterContext(tempfile.TemporaryDirectory())) / "brew.log"
        brew = Path(self.enterContext(tempfile.TemporaryDirectory()))
        (brew / "brew").write_text(recording_shim("brew", log_path))
        (brew / "brew").chmod(0o755)
        completed = self.run_install(machine=("Darwin", "arm64"), shims=[str(brew)])
        self.assertEqual(completed.returncode, 0, completed.stderr)
        ran = log_path.read_text().splitlines()
        tap = assigned_value("homebrew_tap")
        self.assertEqual(ran[0], f"brew tap {tap}")
        self.assertEqual(ran[1], "brew trust --help")
        self.assertEqual(ran[2], f"brew trust --formula {tap}/internkim")
        self.assertEqual(ran[3], "brew install internkim")
        self.assertIn("sudo internkim install", completed.stdout)

    def test_a_homebrew_with_no_trust_gate_is_not_asked_to_trust_anything(self):
        """`brew trust` arrived in Homebrew 7, and a Mac carrying an older one
        has no gate to open. Asking it anyway would stop an install that was
        about to work."""
        log_path = Path(self.enterContext(tempfile.TemporaryDirectory())) / "brew.log"
        brew = Path(self.enterContext(tempfile.TemporaryDirectory()))
        (brew / "brew").write_text(
            "#!/bin/sh\n"
            f'printf "%s\\n" "brew $*" >> "{log_path}"\n'
            'if [ "$1" = trust ]; then echo "Error: Unknown command: trust" >&2; exit 1; fi\n'
        )
        (brew / "brew").chmod(0o755)
        completed = self.run_install(machine=("Darwin", "arm64"), shims=[str(brew)])
        self.assertEqual(completed.returncode, 0, completed.stderr)
        ran = log_path.read_text().splitlines()
        self.assertEqual(ran, [
            f"brew tap {assigned_value('homebrew_tap')}",
            "brew trust --help",
            "brew install internkim",
        ])

    def test_a_mac_with_homebrew_is_never_offered_the_bare_binary(self):
        """A bare binary on a Mac is a company host with no database, no cache
        and nothing brew knows about, which is the state this branch exists to
        avoid."""
        log_path = Path(self.enterContext(tempfile.TemporaryDirectory())) / "brew.log"
        brew = Path(self.enterContext(tempfile.TemporaryDirectory()))
        # The tap succeeds and the install fails, which is the case whose undo
        # is not obvious: the machine now carries a tap nobody asked for.
        (brew / "brew").write_text(
            "#!/bin/sh\n"
            f'printf "%s\\n" "brew $*" >> "{log_path}"\n'
            'if [ "$1" = install ]; then exit 1; fi\n'
        )
        (brew / "brew").chmod(0o755)
        completed = self.run_install(machine=("Darwin", "arm64"), shims=[str(brew)])
        self.assertEqual(completed.returncode, 1)
        self.assertIn("brew untap", completed.stderr)

    def test_a_mac_without_homebrew_is_told_where_to_get_it_and_given_nothing(self):
        completed = self.run_install(machine=("Darwin", "arm64"))
        self.assertEqual(completed.returncode, 1, completed.stdout)
        self.assertIn("https://brew.sh", completed.stderr)
        self.assertIn("Nothing on this machine was changed", completed.stderr)

    def test_a_linux_machine_with_no_supported_package_manager_is_refused_by_name(self):
        """A binary on its own cannot install the host, so a machine without
        apt, dnf or pacman is told which managers the package is published for
        and is given nothing."""
        completed = self.run_install(machine=("Linux", "x86_64"))
        self.assertEqual(completed.returncode, 1, completed.stdout)
        self.assertIn("apt, dnf or pacman", completed.stderr)
        self.assertIn("Nothing on this machine was changed", completed.stderr)

    def test_refuses_a_product_it_publishes_no_build_for(self):
        completed = subprocess.run(
            ["sh", str(install_script), "something-else"], capture_output=True, text=True
        )
        self.assertEqual(completed.returncode, 1)
        self.assertIn("-- host", completed.stderr)

    def linux_machine(self, manager, failing=False, sandbox_installs=True):
        """A machine that has one package manager, sudo and a sandbox for what root writes."""
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.sandbox = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.manager_log = directory / "manager.log"
        (directory / manager).write_text(
            "#!/bin/sh\n"
            f'printf "%s\\n" "{manager} $*" >> "{self.manager_log}"\n'
            + ('[ "$1" = update ] || exit 100\n' if failing else "")
            + "exit 0\n"
        )
        (directory / "sudo").write_text('#!/bin/sh\n[ "$1" = "-v" ] && exit 0\nexec "$@"\n')
        shimmed = [manager, "sudo"]
        if sandbox_installs:
            (directory / "install").write_text(install_shim)
            shimmed.append("install")
        for name in shimmed:
            (directory / name).chmod(0o755)
        return str(directory)

    def serve_host_release(self, tamper=None):
        """A GitHub release's download directory: every package and its SHA256SUMS."""
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        lines = []
        for name in declared_asset_names().values():
            contents = f"{name} bytes\n".encode()
            (directory / name).write_bytes(contents)
            lines.append(f"{hashlib.sha256(contents).hexdigest()}  {name}\n")
        checksums = "".join(lines)
        (directory / "SHA256SUMS").write_text(tamper(checksums) if tamper else checksums)
        handler = functools.partial(QuietRequestHandler, directory=str(directory))
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        return f"http://127.0.0.1:{server.server_address[1]}"

    def run_host_install(self, shims, machine=("Linux", "aarch64"), arguments=(), extra_environment=None, first=()):
        environment = dict(os.environ)
        environment["INTERNKIM_TEST_SANDBOX"] = str(self.sandbox)
        environment.update(extra_environment or {})
        environment["PATH"] = os.pathsep.join([*first, shims, self.uname_shim(*machine), self.path_without_a_package_manager()])
        return subprocess.run(
            ["sh", str(install_script), "host", *arguments], capture_output=True, text=True, env=environment)

    def test_each_manager_installs_the_releases_file_for_its_format_and_architecture(self):
        names = declared_asset_names()
        for manager, suffix, command in [
            ("apt-get", ".deb", "apt-get install -y"),
            ("dnf", ".rpm", "dnf install -y"),
            ("pacman", ".pkg.tar.zst", "pacman -U --needed --noconfirm"),
        ]:
            for machine_architecture, architecture in [("aarch64", "arm64"), ("x86_64", "amd64")]:
                with self.subTest(manager=manager, architecture=machine_architecture):
                    base_url = self.serve_host_release()
                    shims = self.linux_machine(manager)
                    completed = self.run_host_install(
                        shims, machine=("Linux", machine_architecture),
                        extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": base_url})
                    self.assertEqual(completed.returncode, 0, completed.stderr)
                    ran = [line for line in self.manager_log.read_text().splitlines() if line.startswith(command)]
                    self.assertEqual(len(ran), 1, self.manager_log.read_text())
                    self.assertTrue(ran[0].endswith("/" + names[(architecture, suffix)]), ran[0])
                    self.assertIn("sudo internkim install", completed.stdout)

    def test_apt_refreshes_its_indices_before_resolving_the_packages_dependencies(self):
        shims = self.linux_machine("apt-get")
        completed = self.run_host_install(
            shims, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": self.serve_host_release()})
        self.assertEqual(completed.returncode, 0, completed.stderr)
        calls = self.manager_log.read_text().splitlines()
        self.assertEqual(calls[0], "apt-get update")
        self.assertTrue(calls[1].startswith("apt-get install -y /"), calls)

    def test_the_install_adds_no_source_key_or_repository_to_the_machine(self):
        """The package is a file the manager installs; re-running the line is the
        upgrade, so nothing is left behind that the manager would poll."""
        for manager in ("apt-get", "dnf", "pacman"):
            with self.subTest(manager=manager):
                shims = self.linux_machine(manager)
                completed = self.run_host_install(
                    shims, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": self.serve_host_release()})
                self.assertEqual(completed.returncode, 0, completed.stderr)
                written = sorted(path.relative_to(self.sandbox).as_posix() for path in self.sandbox.rglob("*") if path.is_file())
                self.assertEqual(written, ["var/lib/internkim/release-channel"])

    def test_the_install_records_the_channel_it_followed(self):
        """The agent offers an update only to a host that follows stable, and the
        channel exists nowhere else on the machine once this line has run."""
        for arguments, recorded in [((), "stable\n"), (("--channel", "testing"), "testing\n")]:
            with self.subTest(arguments=arguments):
                shims = self.linux_machine("apt-get")
                completed = self.run_host_install(
                    shims, arguments=arguments, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": self.serve_host_release()})
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertEqual((self.sandbox / "var/lib/internkim/release-channel").read_text(), recorded)

    def test_a_package_that_does_not_hash_to_the_published_checksum_is_refused(self):
        def tampered(checksums):
            return checksums.replace(hashlib.sha256(b"internkim-arm64.deb bytes\n").hexdigest(), "0" * 64)

        shims = self.linux_machine("apt-get")
        completed = self.run_host_install(
            shims, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": self.serve_host_release(tampered)})
        self.assertEqual(completed.returncode, 1, completed.stdout)
        self.assertIn("internkim-arm64.deb hashes to", completed.stderr)
        self.assertIn("0" * 64, completed.stderr)
        self.assertFalse(self.manager_log.exists(), "the manager was asked to install something that failed its check")

    def test_a_checksum_list_that_does_not_name_the_package_is_refused(self):
        def without_the_deb(checksums):
            return "".join(line for line in checksums.splitlines(True) if not line.endswith("  internkim-arm64.deb\n"))

        shims = self.linux_machine("apt-get")
        completed = self.run_host_install(
            shims, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": self.serve_host_release(without_the_deb)})
        self.assertEqual(completed.returncode, 1, completed.stdout)
        self.assertIn("lists no internkim-arm64.deb", completed.stderr)
        self.assertFalse(self.manager_log.exists())

    def test_only_the_line_naming_the_package_exactly_answers_for_it(self):
        def renamed(checksums):
            return checksums.replace("  internkim-arm64.deb\n", "  old-internkim-arm64.deb\n")

        shims = self.linux_machine("apt-get")
        completed = self.run_host_install(
            shims, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": self.serve_host_release(renamed)})
        self.assertEqual(completed.returncode, 1, completed.stdout)
        self.assertIn("lists no internkim-arm64.deb", completed.stderr)
        self.assertFalse(self.manager_log.exists())

    def test_a_release_out_of_reach_changes_nothing(self):
        shims = self.linux_machine("dnf")
        completed = self.run_host_install(shims, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": "http://127.0.0.1:9"})
        self.assertEqual(completed.returncode, 1)
        self.assertIn("Could not fetch http://127.0.0.1:9/SHA256SUMS", completed.stderr)
        self.assertIn("Nothing on this machine was changed", completed.stderr)
        self.assertFalse(self.manager_log.exists())

    def test_a_failed_install_points_at_the_managers_own_output(self):
        shims = self.linux_machine("apt-get", failing=True)
        completed = self.run_host_install(
            shims, extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": self.serve_host_release()})
        self.assertEqual(completed.returncode, 1)
        self.assertIn("Installing internkim-arm64.deb failed", completed.stderr)

    def test_a_machine_of_another_architecture_is_refused_before_anything_is_fetched(self):
        shims = self.linux_machine("pacman")
        completed = self.run_host_install(
            shims, machine=("Linux", "riscv64"), extra_environment={"INTERNKIM_INSTALL_RELEASE_URL": "http://127.0.0.1:9"})
        self.assertEqual(completed.returncode, 1)
        self.assertIn("published for arm64 and amd64, and this machine is riscv64", completed.stderr)
        self.assertNotIn("Could not fetch", completed.stderr)
        self.assertFalse(self.manager_log.exists())

    def github(self, newest_tag="v2026.10.01.090507"):
        """A curl that answers GitHub's addresses from a release on disk, and
        records every address it was asked for."""
        served = Path(self.enterContext(tempfile.TemporaryDirectory()))
        lines = []
        for name in declared_asset_names().values():
            (served / name).write_bytes(name.encode())
            lines.append(f"{hashlib.sha256(name.encode()).hexdigest()}  {name}\n")
        (served / "SHA256SUMS").write_text("".join(lines))
        (served / "releases.json").write_text(
            '[\n  {\n    "url": "https://api.github.com/repos/yeomyeonggeori/internkim/releases/1",\n'
            '    "author": {\n      "login": "sample"\n    },\n'
            f'    "tag_name": "{newest_tag}",\n    "prerelease": true\n  }}\n]\n'
        )
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.requested = directory / "requested.log"
        self.curl_options_log = directory / "options.log"
        shim = directory / "curl"
        shim.write_text(
            "#!/bin/sh\n"
            'output=""; address=""; options=""\n'
            'while [ $# -gt 0 ]; do\n'
            '  case "$1" in -o) output="$2"; shift 2 ;; --retry|--retry-delay|--speed-limit|--speed-time) options="$options $1 $2"; shift 2 ;; -*) shift ;; *) address="$1"; shift ;; esac\n'
            "done\n"
            f'printf "%s\\n" "$address" >> "{self.requested}"\n'
            f'printf "%s\\n" "$options" >> "{self.curl_options_log}"\n'
            'case "$address" in\n'
            f'  "{github_newest_release}") file=releases.json ;;\n'
            f'  "{github_downloads}/latest/download/"*|"{github_downloads}/download/{newest_tag}/"*) file="${{address##*/}}" ;;\n'
            '  *) echo "curl: (22) The requested URL returned error: 404" >&2; exit 22 ;;\n'
            "esac\n"
            f'if [ -n "$output" ]; then cp "{served}/$file" "$output"; else cat "{served}/$file"; fi\n'
        )
        shim.chmod(0o755)
        return str(directory)

    def requested_addresses(self):
        return self.requested.read_text().splitlines()

    def test_every_download_gives_up_on_a_stalled_transfer(self):
        shims = self.linux_machine("apt-get")
        completed = self.run_host_install(shims, first=[self.github()])
        self.assertEqual(completed.returncode, 0, completed.stderr)
        for options in self.curl_options_log.read_text().splitlines():
            self.assertIn("--retry 3", options)
            self.assertIn("--speed-limit 1024 --speed-time 60", options)

    def test_stable_installs_from_the_latest_release(self):
        shims = self.linux_machine("apt-get")
        completed = self.run_host_install(shims, first=[self.github()])
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(self.requested_addresses(), [
            f"{github_downloads}/latest/download/SHA256SUMS",
            f"{github_downloads}/latest/download/internkim-arm64.deb",
        ])

    def test_testing_installs_from_the_newest_release_whatever_its_kind(self):
        for arguments in [("--channel", "testing"), ("--channel=testing",)]:
            with self.subTest(arguments=arguments):
                shims = self.linux_machine("dnf")
                completed = self.run_host_install(shims, arguments=arguments, first=[self.github(newest_tag="v9")])
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertEqual(self.requested_addresses(), [
                    github_newest_release,
                    f"{github_downloads}/download/v9/SHA256SUMS",
                    f"{github_downloads}/download/v9/internkim-arm64.rpm",
                ])

    def test_a_pinned_version_installs_that_release_and_lets_apt_go_back_to_it(self):
        shims = self.linux_machine("apt-get")
        completed = self.run_host_install(shims, arguments=("--version", "v7"), first=[self.github(newest_tag="v7")])
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(self.requested_addresses(), [
            f"{github_downloads}/download/v7/SHA256SUMS",
            f"{github_downloads}/download/v7/internkim-arm64.deb",
        ])
        self.assertTrue(self.manager_log.read_text().splitlines()[1].startswith("apt-get install -y --allow-downgrades /"))

    def test_a_pinned_version_older_than_the_installed_one_is_a_dnf_downgrade(self):
        for installed, command in [("9", "downgrade"), ("7", "install"), (None, "install")]:
            with self.subTest(installed=installed):
                shims = self.linux_machine("dnf")
                rpm_answer = '[ "$1" = -qp ] && { echo 7; exit 0; }\n' + (f"echo {installed}\n" if installed else "exit 1\n")
                (Path(shims) / "rpm").write_text("#!/bin/sh\n" + rpm_answer)
                (Path(shims) / "rpm").chmod(0o755)
                completed = self.run_host_install(shims, arguments=("--version=v7",), first=[self.github(newest_tag="v7")])
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertTrue(self.manager_log.read_text().startswith(f"dnf {command} -y /"), self.manager_log.read_text())

    def test_a_pinned_version_that_is_not_a_tag_is_refused_before_anything_is_fetched(self):
        shims = self.linux_machine("apt-get")
        github = self.github()
        for arguments in [("--version", "latest"), ("--version",), ("--version", "2026.10.01")]:
            with self.subTest(arguments=arguments):
                completed = self.run_host_install(shims, arguments=arguments, first=[github])
                self.assertEqual(completed.returncode, 1, completed.stdout)
                self.assertFalse(self.requested.exists())
                self.assertFalse(self.manager_log.exists())

    def test_a_mac_is_never_pinned_to_a_release_the_tap_does_not_carry(self):
        log_path = Path(self.enterContext(tempfile.TemporaryDirectory())) / "brew.log"
        brew = Path(self.enterContext(tempfile.TemporaryDirectory()))
        (brew / "brew").write_text(recording_shim("brew", log_path))
        (brew / "brew").chmod(0o755)
        environment = dict(os.environ)
        environment["PATH"] = os.pathsep.join([str(brew), self.uname_shim("Darwin", "arm64"), self.path_without_a_package_manager()])
        completed = subprocess.run(["sh", str(install_script), "host", "--version", "v7"], capture_output=True, text=True, env=environment)
        self.assertEqual(completed.returncode, 1, completed.stdout)
        self.assertFalse(log_path.exists())

    def test_a_channel_nobody_publishes_is_refused_before_anything_is_fetched(self):
        shims = self.linux_machine("apt-get")
        github = self.github()
        for arguments in [("--channel", "beta"), ("--channel",), ("--chanel", "testing")]:
            with self.subTest(arguments=arguments):
                completed = self.run_host_install(shims, arguments=arguments, first=[github])
                self.assertEqual(completed.returncode, 1, completed.stdout)
                self.assertFalse(self.requested.exists())
                self.assertFalse(self.manager_log.exists())

    def test_testing_with_no_release_github_will_name_changes_nothing(self):
        shims = self.linux_machine("apt-get")
        github = self.github()
        (Path(github) / "curl").write_text("#!/bin/sh\necho '[]'\n")
        completed = self.run_host_install(shims, arguments=("--channel", "testing"), first=[github])
        self.assertEqual(completed.returncode, 1, completed.stdout)
        self.assertIn(f"Could not read the newest release from {github_newest_release}", completed.stderr)
        self.assertFalse(self.manager_log.exists())

if __name__ == "__main__":
    unittest.main()
