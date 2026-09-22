import hashlib
import functools
import http.server
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import unittest


repository_root = Path(__file__).resolve().parents[2]
install_script = repository_root / "web/static/install.sh"
companion_address = repository_root / "web/static/companion/install.sh"
keyring_path = "/usr/share/keyrings/internkim-archive-keyring.pgp"
apt_source_path = "/etc/apt/sources.list.d/internkim.sources"
published_keyring = b"-----BEGIN PGP PUBLIC KEY BLOCK-----\nnot a real key\n"

# `install` is the only command the script uses to put a file where root can
# see it, and its destination is always the last argument. Rewriting that one
# argument under a sandbox is what lets this test read what the script wrote to
# /etc and /usr/share without the test machine having either path touched.
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
def published_binary(binary_name):
    return f"#!/bin/sh\necho {binary_name}\n".encode()


class PublishedRelease:
    def __init__(self, directory, checksum_of):
        self.directory = Path(directory)
        for product in ("companion", "host"):
            for operating_system in ("darwin", "linux"):
                for architecture in ("arm64", "amd64"):
                    self.publish(product, f"internkim-{product}-{operating_system}-{architecture}", checksum_of)

    def publish(self, product, binary_name, checksum_of):
        prefix = self.directory / product / "latest"
        prefix.mkdir(parents=True, exist_ok=True)
        content = published_binary(binary_name)
        (prefix / binary_name).write_bytes(content)
        checksums = prefix / "SHA256SUMS"
        line = f"{checksum_of(content)}  {binary_name}\n"
        with checksums.open("a") as output:
            output.write(line)


class QuietRequestHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, format, *arguments):
        pass


class InstallScriptTests(unittest.TestCase):
    def serve(self, checksum_of=lambda content: hashlib.sha256(content).hexdigest()):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.served_directory = directory
        (directory / "install.sh").write_bytes(install_script.read_bytes())
        PublishedRelease(directory, checksum_of)
        handler = functools.partial(QuietRequestHandler, directory=str(directory))
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        return f"http://127.0.0.1:{server.server_address[1]}"

    def run_install(self, product, base_url, machine, bin_dir=None, shims=()):
        environment = dict(os.environ)
        environment["INTERNKIM_INSTALL_BIN_DIR"] = str(bin_dir or self.enterContext(tempfile.TemporaryDirectory()))
        directories = [*shims, self.uname_shim(*machine)]
        environment["PATH"] = os.pathsep.join([*directories, self.path_without_a_package_manager()])
        environment["INTERNKIM_INSTALL_RELEASE_URL"] = f"{base_url}/{product}/latest"
        completed = subprocess.run(
            ["sh", str(install_script), product],
            capture_output=True,
            text=True,
            env=environment,
        )
        return completed, Path(environment["INTERNKIM_INSTALL_BIN_DIR"])

    def path_without_a_package_manager(self):
        """A PATH on which the script can find neither apt-get nor brew.

        `host` reaches for a package manager whenever one is there, so the
        direct-download path these tests read is what a machine without one
        takes. On this Mac that means hiding brew; on a Debian build machine it
        means hiding apt-get, and a test that passed only where it was written
        proves nothing.
        """
        return os.pathsep.join(
            directory
            for directory in os.environ["PATH"].split(os.pathsep)
            if directory and not any((Path(directory) / name).exists() for name in ("apt-get", "brew"))
        )

    def forget_checksum_line(self, base_url, product, binary_name):
        checksums = self.served_directory / product / "latest" / "SHA256SUMS"
        kept = [line for line in checksums.read_text().splitlines(True) if not line.endswith(f"  {binary_name}\n")]
        checksums.write_text("".join(kept))

    def silent_hasher(self):
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        for name in ("sha256sum", "shasum"):
            shim = directory / name
            shim.write_text("#!/bin/sh\nexit 0\n")
            shim.chmod(0o755)
        return str(directory)

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

    def test_installs_the_build_for_this_machine(self):
        base_url = self.serve()
        for product in ("companion", "host"):
            for reported, expected in [
                (("Darwin", "arm64"), "darwin-arm64"),
                (("Darwin", "x86_64"), "darwin-amd64"),
                (("Linux", "aarch64"), "linux-arm64"),
                (("Linux", "x86_64"), "linux-amd64"),
            ]:
                with self.subTest(product=product, machine=reported):
                    completed, bin_dir = self.run_install(product, base_url, machine=reported)
                    self.assertEqual(completed.returncode, 0, completed.stderr)
                    installed = bin_dir / f"internkim-{product}"
                    self.assertEqual(installed.read_bytes(), published_binary(f"internkim-{product}-{expected}"))

    def test_a_mac_with_homebrew_installs_the_formula_from_the_tap(self):
        """The published one line has to leave a registered Homebrew install, or
        `brew upgrade` and `brew uninstall` have nothing to act on afterwards."""
        base_url = self.serve()
        log_path = Path(self.enterContext(tempfile.TemporaryDirectory())) / "brew.log"
        brew = Path(self.enterContext(tempfile.TemporaryDirectory()))
        (brew / "brew").write_text(recording_shim("brew", log_path))
        (brew / "brew").chmod(0o755)
        completed, bin_dir = self.run_install(
            "host", base_url, machine=("Darwin", "arm64"), shims=[str(brew)])
        self.assertEqual(completed.returncode, 0, completed.stderr)
        ran = log_path.read_text().splitlines()
        self.assertEqual(ran[0], "brew tap yeomyeonggeori/internkim")
        self.assertEqual(ran[1], "brew install internkim")
        self.assertIn("sudo internkim install", completed.stdout)
        self.assertFalse((bin_dir / "internkim-host").exists(), "a Mac with Homebrew should get the formula, not a bare binary")

    def test_a_mac_with_homebrew_is_never_offered_the_bare_binary(self):
        """A bare binary on a Mac is a company host with no database, no cache
        and nothing brew knows about, which is the state this branch exists to
        avoid."""
        base_url = self.serve()
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
        completed, bin_dir = self.run_install(
            "host", base_url, machine=("Darwin", "arm64"), shims=[str(brew)])
        self.assertEqual(completed.returncode, 1)
        self.assertIn("brew untap", completed.stderr)
        self.assertFalse((bin_dir / "internkim-host").exists())

    def test_a_mac_without_homebrew_is_told_where_to_get_it(self):
        base_url = self.serve()
        completed, bin_dir = self.run_install("host", base_url, machine=("Darwin", "arm64"))
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertIn("https://brew.sh", completed.stdout)
        self.assertTrue((bin_dir / "internkim-host").exists())

    def test_refuses_a_machine_with_no_published_build(self):
        base_url = self.serve()
        for reported in [("Darwin", "riscv64"), ("OpenBSD", "arm64")]:
            with self.subTest(machine=reported):
                completed, bin_dir = self.run_install("host", base_url, machine=reported)
                self.assertEqual(completed.returncode, 1)
                self.assertIn("has no build for", completed.stderr)
                self.assertFalse((bin_dir / "internkim-host").exists())

    def test_refuses_a_download_whose_checksum_was_not_published(self):
        base_url = self.serve(checksum_of=lambda content: hashlib.sha256(content + b"tampered").hexdigest())
        completed, bin_dir = self.run_install("host", base_url, machine=("Linux", "x86_64"))
        self.assertEqual(completed.returncode, 1)
        self.assertIn("does not match the published checksum", completed.stderr)
        self.assertFalse((bin_dir / "internkim-host").exists())

    def test_refuses_a_download_the_checksum_list_does_not_name(self):
        base_url = self.serve()
        self.forget_checksum_line(base_url, "host", "internkim-host-linux-amd64")
        completed, bin_dir = self.run_install("host", base_url, machine=("Linux", "x86_64"))
        self.assertEqual(completed.returncode, 1, completed.stderr)
        self.assertIn("does not match the published checksum", completed.stderr)
        self.assertFalse((bin_dir / "internkim-host").exists())

    def test_refuses_a_download_when_nothing_can_be_compared(self):
        base_url = self.serve()
        self.forget_checksum_line(base_url, "host", "internkim-host-linux-amd64")
        completed, bin_dir = self.run_install(
            "host", base_url, machine=("Linux", "x86_64"), shims=[self.silent_hasher()]
        )
        self.assertEqual(completed.returncode, 1, completed.stderr)
        self.assertIn("does not match the published checksum", completed.stderr)
        self.assertFalse((bin_dir / "internkim-host").exists())

    def test_the_address_devices_still_print_installs_the_companion(self):
        base_url = self.serve()
        bin_dir = Path(self.enterContext(tempfile.TemporaryDirectory()))
        completed = subprocess.run(
            ["sh", str(companion_address)],
            capture_output=True,
            text=True,
            env={
                **os.environ,
                "PATH": self.uname_shim("Linux", "x86_64") + os.pathsep + os.environ["PATH"],
                "INTERNKIM_INSTALL_SCRIPT_URL": f"{base_url}/install.sh",
                "INTERNKIM_INSTALL_BIN_DIR": str(bin_dir),
                "INTERNKIM_INSTALL_RELEASE_URL": f"{base_url}/companion/latest",
            },
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(
            (bin_dir / "internkim-companion").read_bytes(),
            published_binary("internkim-companion-linux-amd64"),
        )

    def test_the_address_devices_still_print_fails_loudly_when_it_cannot_be_served(self):
        base_url = self.serve()
        bin_dir = Path(self.enterContext(tempfile.TemporaryDirectory()))
        completed = subprocess.run(
            ["sh", str(companion_address)],
            capture_output=True,
            text=True,
            env={
                **os.environ,
                "INTERNKIM_INSTALL_SCRIPT_URL": f"{base_url}/nothing-here.sh",
                "INTERNKIM_INSTALL_BIN_DIR": str(bin_dir),
            },
        )
        self.assertNotEqual(completed.returncode, 0)
        self.assertFalse((bin_dir / "internkim-companion").exists())

    def debian_machine(self, failing_apt_subcommand="", sandbox_installs=True):
        """A machine that has apt-get, dpkg and sudo, and a sandbox for what root writes."""
        directory = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.sandbox = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.apt_log = directory / "apt.log"
        (directory / "apt-get").write_text(
            "#!/bin/sh\n"
            f'printf "%s\\n" "apt-get $*" >> "{self.apt_log}"\n'
            f'case "$1" in {failing_apt_subcommand or "__never__"}) exit 100 ;; esac\n'
            "exit 0\n"
        )
        (directory / "dpkg").write_text(recording_shim("dpkg", directory / "apt.log", output="arm64"))
        (directory / "sudo").write_text('#!/bin/sh\n[ "$1" = "-v" ] && exit 0\nexec "$@"\n')
        shimmed = ["apt-get", "dpkg", "sudo"]
        if sandbox_installs:
            (directory / "install").write_text(install_shim)
            shimmed.append("install")
        for name in shimmed:
            (directory / name).chmod(0o755)
        return str(directory)

    def run_install_on_debian(self, base_url, shims, product="host"):
        environment = dict(os.environ)
        environment["INTERNKIM_TEST_SANDBOX"] = str(self.sandbox)
        environment["INTERNKIM_INSTALL_BIN_DIR"] = str(self.enterContext(tempfile.TemporaryDirectory()))
        environment["INTERNKIM_INSTALL_REPOSITORY_URL"] = f"{base_url}/deb"
        environment["INTERNKIM_INSTALL_RELEASE_URL"] = f"{base_url}/{product}/latest"
        environment["PATH"] = os.pathsep.join([shims, self.uname_shim("Linux", "aarch64"), environment["PATH"]])
        completed = subprocess.run(
            ["sh", str(install_script), product],
            capture_output=True,
            text=True,
            env=environment,
        )
        return completed, Path(environment["INTERNKIM_INSTALL_BIN_DIR"])

    def publish_keyring(self, base_url):
        (self.served_directory / "deb").mkdir(parents=True, exist_ok=True)
        (self.served_directory / "deb" / "internkim-archive-keyring.pgp").write_bytes(published_keyring)
        return base_url

    def test_a_machine_with_apt_gets_the_package_and_not_a_bare_binary(self):
        base_url = self.publish_keyring(self.serve())
        completed, bin_dir = self.run_install_on_debian(base_url, self.debian_machine())
        self.assertEqual(completed.returncode, 0, completed.stderr)

        written = (self.sandbox / apt_source_path.lstrip("/")).read_text()
        self.assertEqual(
            written.splitlines(),
            [
                "Types: deb",
                f"URIs: {base_url}/deb",
                "Suites: stable",
                "Components: main",
                "Architectures: arm64",
                f"Signed-By: {keyring_path}",
            ],
        )
        self.assertEqual((self.sandbox / keyring_path.lstrip("/")).read_bytes(), published_keyring)

        calls = self.apt_log.read_text().splitlines()
        self.assertIn("apt-get update", calls)
        self.assertIn("apt-get install -y internkim", calls)
        self.assertEqual(list(bin_dir.iterdir()), [])

    def test_the_key_never_lands_where_it_would_sign_every_repository(self):
        base_url = self.publish_keyring(self.serve())
        completed, _ = self.run_install_on_debian(base_url, self.debian_machine())
        self.assertEqual(completed.returncode, 0, completed.stderr)
        for forbidden in ("etc/apt/trusted.gpg", "etc/apt/trusted.gpg.d"):
            self.assertFalse((self.sandbox / forbidden).exists(), forbidden)
        self.assertNotIn("apt-key", self.apt_log.read_text())

    def test_a_failed_package_install_says_how_to_undo_what_it_wrote(self):
        base_url = self.publish_keyring(self.serve())
        completed, _ = self.run_install_on_debian(base_url, self.debian_machine(failing_apt_subcommand="install"))
        self.assertEqual(completed.returncode, 1)
        self.assertIn(apt_source_path, completed.stderr)
        self.assertIn(keyring_path, completed.stderr)

    def test_a_signing_key_that_cannot_be_fetched_changes_nothing(self):
        base_url = self.serve()
        completed, _ = self.run_install_on_debian(base_url, self.debian_machine())
        self.assertEqual(completed.returncode, 1)
        self.assertIn("signing key", completed.stderr)
        self.assertFalse((self.sandbox / apt_source_path.lstrip("/")).exists())
        self.assertNotIn("apt-get", self.apt_log.read_text())

    def test_the_companion_ignores_the_package_manager(self):
        base_url = self.publish_keyring(self.serve())
        completed, bin_dir = self.run_install_on_debian(
            base_url, self.debian_machine(sandbox_installs=False), product="companion"
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(
            (bin_dir / "internkim-companion").read_bytes(),
            published_binary("internkim-companion-linux-arm64"),
        )
        self.assertFalse(self.apt_log.exists())

    def test_refuses_a_product_it_publishes_no_build_for(self):
        completed = subprocess.run(
            ["sh", str(install_script), "something-else"], capture_output=True, text=True
        )
        self.assertEqual(completed.returncode, 1)
        self.assertIn("companion|host", completed.stderr)

    def test_a_second_install_replaces_the_binary_in_place(self):
        base_url = self.serve()
        bin_dir = Path(self.enterContext(tempfile.TemporaryDirectory()))
        first, _ = self.run_install("host", base_url, machine=("Linux", "x86_64"), bin_dir=bin_dir)
        self.assertEqual(first.returncode, 0, first.stderr)
        second, _ = self.run_install("host", base_url, machine=("Linux", "x86_64"), bin_dir=bin_dir)
        self.assertEqual(second.returncode, 0, second.stderr)
        self.assertEqual((bin_dir / "internkim-host").read_bytes(), published_binary("internkim-host-linux-amd64"))
        self.assertEqual(sorted(path.name for path in bin_dir.iterdir()), ["internkim-host"])


if __name__ == "__main__":
    unittest.main()
