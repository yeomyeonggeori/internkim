"""The parts of the native-install rig that run on this Mac.

Building a Debian package and an apt repository, and driving one disposable
arm64 Debian guest. The assertions that read the guest live in
`tools/test-native-install`; everything here is what they need in order to have
a machine and something to install on it.
"""

import gzip
import hashlib
import io
import json
import posixpath
import shutil
import subprocess
import tarfile
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parent.parent
KERNEL_IMAGE_PATH = REPOSITORY_ROOT / ".dependency" / "container-kernel" / "Image-6.1.68-kvm"
BASE_IMAGE = "debian:trixie-slim"
ARCHITECTURE = "arm64"
SUITE = "stable"
COMPONENT = "main"
PACKAGE_NAME = "internkim"
SHARE_PATH = "/srv/internkim-rig"
APT_SOURCE_PATH = "/etc/apt/sources.list.d/internkim.sources"
KEYRING_PATH = "/usr/share/keyrings/internkim-archive-keyring.pgp"
STATE_DIRECTORY = "/var/lib/internkim"
CONFIGURATION_DIRECTORY = "/etc/internkim"
STAND_IN_ADMIND_PORT = 8080
STAND_IN_RELAY_PORT = 8081


class RigFailure(Exception):
    pass


def run(arguments, **keywords):
    return subprocess.run(arguments, capture_output=True, text=True, **keywords)


def checked(arguments, **keywords):
    completed = run(arguments, **keywords)
    if completed.returncode != 0:
        raise RigFailure(f"{' '.join(arguments)} failed: {completed.stderr.strip() or completed.stdout.strip()}")
    return completed


def read_archive_member(archive_path, wanted_name):
    data = Path(archive_path).read_bytes()
    if not data.startswith(b"!<arch>\n"):
        raise RigFailure(f"{archive_path} is not an ar archive, so it is not a .deb")
    offset = 8
    while offset + 60 <= len(data):
        header = data[offset : offset + 60]
        name = header[0:16].decode("ascii", "replace").strip().rstrip("/")
        size = int(header[48:58].decode("ascii", "replace").strip())
        body = data[offset + 60 : offset + 60 + size]
        if name == wanted_name or name.startswith(wanted_name + "."):
            return name, body
        offset += 60 + size + size % 2
    return None, None


def write_archive(destination, members):
    with Path(destination).open("wb") as archive:
        archive.write(b"!<arch>\n")
        for name, payload in members:
            archive.write(
                f"{name:<16}{0:<12}{0:<6}{0:<6}{'100644':<8}{len(payload):<10}".encode("ascii") + b"`\n"
            )
            archive.write(payload)
            if len(payload) % 2:
                archive.write(b"\n")


def package_fields(package_path):
    _, control_archive = read_archive_member(package_path, "control.tar")
    if control_archive is None:
        raise RigFailure(f"{package_path} carries no control member")
    with tarfile.open(fileobj=io.BytesIO(control_archive), mode="r:*") as archive:
        for member in archive.getmembers():
            if Path(member.name).name != "control":
                continue
            return parse_control_paragraph(archive.extractfile(member).read().decode())
    raise RigFailure(f"{package_path} carries no control file")


def dependency_names(depends_field):
    """The names apt can be asked to install, out of a Depends: line.

    A version constraint is dropped and the first alternative of an `a | b`
    clause is taken, which is the one a Debian machine installs by default.
    """
    names = []
    for clause in depends_field.replace("\n", " ").split(","):
        first = clause.split("|")[0].split()
        if first:
            names.append(first[0])
    return names


def parse_control_paragraph(text):
    fields = {}
    name = None
    for line in text.splitlines():
        if not line.strip():
            continue
        if line[0].isspace() and name:
            fields[name] += "\n " + line.strip()
            continue
        name, _, value = line.partition(":")
        fields[name.strip()] = value.strip()
    return fields


class Release:
    """An apt repository on this Mac, served to the guest over HTTP."""

    def __init__(self, directory):
        self.directory = Path(directory)
        self.repository_directory = self.directory / "deb"
        self.keyring_directory = Path(tempfile.mkdtemp(prefix="/tmp/ikrig-"))
        self.public_keyring_path = self.directory / "internkim-archive-keyring.pgp"
        self.published = []
        self.server = None
        self.port = None

    def generate_signing_key(self):
        if shutil.which("gpg") is None:
            raise RigFailure(
                "gpg is not on PATH; the rig signs its repository because an unsigned one would "
                "not exercise the Signed-By path the install is supposed to take"
            )
        self.keyring_directory.chmod(0o700)
        checked(
            [
                "gpg", "--homedir", str(self.keyring_directory), "--batch", "--yes",
                "--passphrase", "", "--quick-generate-key",
                "InternKim Install Rig <rig@invalid.internkim.test>", "default", "default", "never",
            ]
        )
        exported = subprocess.run(
            ["gpg", "--homedir", str(self.keyring_directory), "--batch", "--export"],
            capture_output=True,
        )
        if not exported.stdout:
            raise RigFailure(f"gpg exported an empty keyring: {exported.stderr.decode()}")
        self.public_keyring_path.write_bytes(exported.stdout)

    def publish(self, package_path):
        pool = self.repository_directory / "pool" / COMPONENT / PACKAGE_NAME[0] / PACKAGE_NAME
        pool.mkdir(parents=True, exist_ok=True)
        destination = pool / Path(package_path).name
        shutil.copyfile(package_path, destination)
        self.published.append(destination)
        self.rebuild_indices()
        return destination

    def rebuild_indices(self):
        binary_directory = self.repository_directory / "dists" / SUITE / COMPONENT / f"binary-{ARCHITECTURE}"
        binary_directory.mkdir(parents=True, exist_ok=True)
        paragraphs = []
        for package_path in sorted(self.published):
            fields = package_fields(package_path)
            payload = package_path.read_bytes()
            relative = package_path.relative_to(self.repository_directory)
            lines = [f"{name}: {value}" for name, value in fields.items() if name != "Description"]
            lines += [
                f"Filename: {relative}",
                f"Size: {len(payload)}",
                f"MD5sum: {hashlib.md5(payload).hexdigest()}",
                f"SHA256: {hashlib.sha256(payload).hexdigest()}",
                f"Description: {fields.get('Description', PACKAGE_NAME)}",
            ]
            paragraphs.append("\n".join(lines))
        packages = ("\n\n".join(paragraphs) + "\n").encode()
        (binary_directory / "Packages").write_bytes(packages)
        (binary_directory / "Packages.gz").write_bytes(gzip.compress(packages, mtime=0))
        self.write_release()

    def write_release(self):
        suite_directory = self.repository_directory / "dists" / SUITE
        header = [
            "Origin: InternKim",
            "Label: InternKim",
            f"Suite: {SUITE}",
            f"Codename: {SUITE}",
            f"Architectures: {ARCHITECTURE}",
            f"Components: {COMPONENT}",
            f"Date: {datetime.now(timezone.utc).strftime('%a, %d %b %Y %H:%M:%S UTC')}",
            "Description: the rig's stand-in for updates.intern.kim",
        ]
        indices = sorted(
            path for path in suite_directory.rglob("*") if path.is_file() and path.name not in ("Release", "InRelease", "Release.gpg")
        )
        for algorithm, digest in (("MD5Sum", hashlib.md5), ("SHA256", hashlib.sha256)):
            header.append(f"{algorithm}:")
            for path in indices:
                payload = path.read_bytes()
                header.append(f" {digest(payload).hexdigest()} {len(payload)} {path.relative_to(suite_directory)}")
        release_path = suite_directory / "Release"
        release_path.write_text("\n".join(header) + "\n")
        in_release_path = suite_directory / "InRelease"
        in_release_path.unlink(missing_ok=True)
        checked(
            [
                "gpg", "--homedir", str(self.keyring_directory), "--batch", "--yes",
                "--clearsign", "--output", str(in_release_path), str(release_path),
            ]
        )

    def serve(self):
        import functools
        import http.server
        import threading

        class QuietHandler(http.server.SimpleHTTPRequestHandler):
            def send_head(self):
                del self.headers["If-Modified-Since"]
                del self.headers["If-None-Match"]
                return super().send_head()

            def log_message(self, format, *arguments):
                pass

        handler = functools.partial(QuietHandler, directory=str(self.directory))
        self.server = http.server.ThreadingHTTPServer(("0.0.0.0", 0), handler)
        self.port = self.server.server_address[1]
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        return self.port

    def stop(self):
        run(["gpgconf", "--homedir", str(self.keyring_directory), "--kill", "gpg-agent"])
        shutil.rmtree(self.keyring_directory, ignore_errors=True)
        if self.server:
            self.server.shutdown()
            self.server.server_close()
            self.server = None


def build_stand_in_package(directory, version, revision):
    """A package shaped like the one §2 of the plan describes, and nothing like its contents.

    It exists so the rig's own assertions can be run today. It carries the
    shapes the assertions read — a setuid helper, a conffile, two units with
    readiness endpoints, a postinst that creates the state tree, a postrm that
    keeps it — and none of the product.
    """
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    package_path = directory / f"{PACKAGE_NAME}_{version}_{ARCHITECTURE}.deb"

    control_fields = "\n".join(
        [
            f"Package: {PACKAGE_NAME}",
            f"Version: {version}",
            f"Architecture: {ARCHITECTURE}",
            "Maintainer: InternKim <nobody@invalid.internkim.test>",
            "Section: admin",
            "Priority: optional",
            "Depends: python3, ca-certificates",
            "Description: stand-in for the company host package",
            " Built by tools/test-native-install so the rig can be exercised before",
            " the real package exists. It is not the product.",
        ]
    ) + "\n"

    health_server = STAND_IN_HEALTH_SERVER.replace("@REVISION@", revision)

    data_entries = [
        ("./usr/bin/internkim", 0o755, f"#!/bin/sh\necho {version}\n".encode()),
        ("./usr/lib/internkim/blueclaw-posix-helper", 0o4755, b"#!/bin/sh\nexit 0\n"),
        ("./usr/lib/internkim/health-server", 0o755, health_server.encode()),
        (
            f".{CONFIGURATION_DIRECTORY}/runtime.json",
            0o644,
            json.dumps({"standIn": True}, indent=2).encode() + b"\n",
        ),
        (
            "./lib/systemd/system/internkim-admind.service",
            0o644,
            stand_in_unit("internkim-admind", STAND_IN_ADMIND_PORT, "/admin/api/health").encode(),
        ),
        (
            "./lib/systemd/system/internkim-relay.service",
            0o644,
            stand_in_unit("internkim-relay", STAND_IN_RELAY_PORT, "/_readiness").encode(),
        ),
    ]

    directories = []
    for name, _, _ in data_entries:
        parent = posixpath.dirname(name)
        while parent not in (".", "/", "") and parent not in directories:
            directories.append(parent)
            parent = posixpath.dirname(parent)

    data_archive = io.BytesIO()
    with tarfile.open(fileobj=data_archive, mode="w:gz") as archive:
        for name in sorted(directories):
            information = tarfile.TarInfo(name)
            information.type = tarfile.DIRTYPE
            information.mode = 0o755
            information.mtime = 0
            information.uname = "root"
            information.gname = "root"
            archive.addfile(information)
        for name, mode, payload in data_entries:
            information = tarfile.TarInfo(name)
            information.size = len(payload)
            information.mode = mode
            information.mtime = 0
            information.uname = "root"
            information.gname = "root"
            archive.addfile(information, io.BytesIO(payload))

    control_entries = [
        ("./control", 0o644, control_fields.encode()),
        ("./conffiles", 0o644, f"{CONFIGURATION_DIRECTORY}/runtime.json\n".encode()),
        ("./postinst", 0o755, STAND_IN_POSTINST.encode()),
        ("./prerm", 0o755, STAND_IN_PRERM.encode()),
        ("./postrm", 0o755, STAND_IN_POSTRM.encode()),
    ]
    control_archive = io.BytesIO()
    with tarfile.open(fileobj=control_archive, mode="w:gz") as archive:
        for name, mode, payload in control_entries:
            information = tarfile.TarInfo(name)
            information.size = len(payload)
            information.mode = mode
            information.mtime = 0
            archive.addfile(information, io.BytesIO(payload))

    write_archive(
        package_path,
        [
            ("debian-binary", b"2.0\n"),
            ("control.tar.gz", control_archive.getvalue()),
            ("data.tar.gz", data_archive.getvalue()),
        ],
    )
    return package_path


def stand_in_unit(name, port, path):
    return "\n".join(
        [
            "[Unit]",
            f"Description={name} (install rig stand-in)",
            "After=network.target",
            "",
            "[Service]",
            "Type=simple",
            f"ExecStart=/usr/lib/internkim/health-server {port} {path}",
            "Restart=on-failure",
            "",
            "[Install]",
            "WantedBy=multi-user.target",
            "",
        ]
    )


STAND_IN_HEALTH_SERVER = """#!/usr/bin/python3
import http.server
import json
import sys
import time

port = int(sys.argv[1])
path = sys.argv[2]
started_at = time.time()


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != path:
            self.send_response(404)
            self.end_headers()
            return
        body = json.dumps(
            {"status": "ok", "gitRevision": "@REVISION@", "startedAt": started_at}
        ).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *arguments):
        pass


http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
"""


STAND_IN_POSTINST = f"""#!/bin/sh
set -e
if [ "$1" = configure ]; then
  install -d -m 0700 -o root -g root {STATE_DIRECTORY}
  install -d -m 0700 -o root -g root {STATE_DIRECTORY}/companies
  systemctl daemon-reload
  systemctl enable --now internkim-relay.service internkim-admind.service
  systemctl try-restart internkim-relay.service internkim-admind.service
fi
exit 0
"""


STAND_IN_PRERM = """#!/bin/sh
set -e
if [ "$1" = remove ] || [ "$1" = deconfigure ]; then
  systemctl disable --now internkim-admind.service internkim-relay.service || true
fi
exit 0
"""


STAND_IN_POSTRM = f"""#!/bin/sh
set -e
if [ "$1" = remove ] || [ "$1" = purge ]; then
  systemctl daemon-reload || true
fi
if [ "$1" = purge ]; then
  rm -rf {CONFIGURATION_DIRECTORY}
  if [ -d {STATE_DIRECTORY} ]; then
    echo "Your company is still at {STATE_DIRECTORY}. Removing it loses the keys that sign"
    echo "messages under your people's names, and nothing can recover them."
    echo "Delete it with: internkim destroy --confirm"
  fi
fi
exit 0
"""


BOOTSTRAP_SCRIPT = "\n".join(
    [
        "set -eu",
        "export DEBIAN_FRONTEND=noninteractive",
        "if [ ! -x /lib/systemd/systemd ]; then",
        "  for attempt in 1 2 3; do",
        "    apt-get update && apt-get install -y --no-install-recommends systemd systemd-sysv curl ca-certificates && break",
        "    sleep $((attempt * 3))",
        "  done",
        "fi",
        "if [ -L /etc/resolv.conf ]; then cp /etc/resolv.conf /etc/resolv.conf.static && rm /etc/resolv.conf && mv /etc/resolv.conf.static /etc/resolv.conf; fi",
        "ln -sf /dev/null /etc/systemd/system/systemd-resolved.service",
        "exec /lib/systemd/systemd",
    ]
)

PRESEEDED_PACKAGES = ("systemd", "systemd-sysv", "curl", "ca-certificates")


class Machine:
    """One disposable arm64 Debian guest, created and destroyed by this rig alone."""

    def __init__(self, name, share_directory, container_binary="container"):
        self.name = name
        self.share_directory = Path(share_directory)
        self.container_binary = container_binary
        self.created = False

    def preflight(self):
        if shutil.which(self.container_binary) is None:
            raise RigFailure("the `container` CLI is not on PATH; this rig uses the same runtime the local fleet does")
        if not KERNEL_IMAGE_PATH.exists():
            raise RigFailure(f"{KERNEL_IMAGE_PATH} is missing; run `make prepare-container-kernel`")
        for existing in self.list_containers():
            if existing.get("configuration", {}).get("id") == self.name:
                raise RigFailure(f"a container named {self.name} already exists; the rig refuses to adopt one it did not create")

    def list_containers(self):
        completed = run([self.container_binary, "ls", "-a", "--format", "json"])
        if completed.returncode != 0 or not completed.stdout.strip():
            return []
        try:
            return json.loads(completed.stdout)
        except json.JSONDecodeError:
            return []

    def create(self, cpu_count=2, memory_mebibytes=2048):
        checked(
            [
                self.container_binary, "create",
                "--name", self.name,
                "--cpus", str(cpu_count),
                "--memory", f"{memory_mebibytes}M",
                "--tmpfs", "/run",
                "--tmpfs", "/run/lock",
                "--kernel", str(KERNEL_IMAGE_PATH),
                "--volume", f"{self.share_directory}:{SHARE_PATH}",
                BASE_IMAGE,
                "sh", "-c", BOOTSTRAP_SCRIPT,
            ]
        )
        self.created = True
        checked([self.container_binary, "start", self.name])

    def wait_until_running(self, timeout_seconds=300):
        deadline = time.monotonic() + timeout_seconds
        while time.monotonic() < deadline:
            completed = self.shell("systemctl is-system-running 2>/dev/null || true")
            if completed.stdout.strip() in ("running", "degraded"):
                return completed.stdout.strip()
            time.sleep(3)
        raise RigFailure(f"{self.name} did not reach a running systemd within {timeout_seconds}s")

    def shell(self, script, timeout_seconds=900):
        script_path = self.share_directory / "scripts" / f"step-{hashlib.sha256(script.encode()).hexdigest()[:16]}.sh"
        script_path.parent.mkdir(parents=True, exist_ok=True)
        script_path.write_text("set -o pipefail\n" + script)
        return subprocess.run(
            [self.container_binary, "exec", self.name, "bash", f"{SHARE_PATH}/scripts/{script_path.name}"],
            capture_output=True,
            text=True,
            timeout=timeout_seconds,
        )

    def checked_shell(self, script, **keywords):
        completed = self.shell(script, **keywords)
        if completed.returncode != 0:
            raise RigFailure(f"in the guest: {script.strip().splitlines()[0]}\n{completed.stdout}{completed.stderr}")
        return completed

    def host_gateway(self):
        completed = self.checked_shell(
            "awk '$2 == \"00000000\" { print $3; exit }' /proc/net/route"
        )
        packed = completed.stdout.strip()
        if len(packed) != 8:
            raise RigFailure(f"the guest has no default route: {completed.stdout!r}")
        octets = [int(packed[index : index + 2], 16) for index in (6, 4, 2, 0)]
        return ".".join(str(octet) for octet in octets)

    def destroy(self):
        if not self.created:
            return
        run([self.container_binary, "stop", self.name])
        run([self.container_binary, "rm", "--force", self.name])
        self.created = False
