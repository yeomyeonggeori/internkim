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
        environment["PATH"] = os.pathsep.join([*directories, environment["PATH"]])
        environment["INTERNKIM_INSTALL_RELEASE_URL"] = f"{base_url}/{product}/latest"
        completed = subprocess.run(
            ["sh", str(install_script), product],
            capture_output=True,
            text=True,
            env=environment,
        )
        return completed, Path(environment["INTERNKIM_INSTALL_BIN_DIR"])

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
