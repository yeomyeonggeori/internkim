"""The parts of the native-install rig that run on this Mac.

Building a Debian package and an apt repository, and driving one disposable
arm64 Debian guest. The assertions that read the guest live in
`tools/test-native-install`; everything here is what they need in order to have
a machine and something to install on it.
"""

import hashlib
import io
import json
import posixpath
import shutil
import subprocess
import tarfile
import tempfile
import time
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
# Where anything asks whether a service is up, and which of those answers
# carries the revision the running process reports. Only the admin gateway's
# does: blueclaw's health body names its database, its language model and its
# protocol identity and no revision at all, which is why an upgrade cannot be
# judged from port 8080. `tools/tests/test_native_install_rig.py` reads the Go
# constants and fails when these drift from them.
MESSENGER_READINESS = (3000, "/_readiness")
AGENT_HEALTH = (8080, "/admin/api/health")
ADMIN_GATEWAY_HEALTH = (18080, "/admin/api/health")
READINESS_PROBES = (MESSENGER_READINESS, AGENT_HEALTH, ADMIN_GATEWAY_HEALTH)
REVISION_PROBE = ADMIN_GATEWAY_HEALTH

# The package's one configuration file dpkg protects, and the file every unit
# waits on. Section 5 of the plan makes runtime.json an optional override the
# package does not ship, so editing that would say nothing about whether dpkg
# preserved an operator's work.
CONFFILE_PATH = "/etc/internkim/company-host.env"
CONFFILE_EDIT = "# edited-by-the-rig"
COMPANY_CONDITION_PATH = "/var/lib/internkim/current/host.env"

# The repository's own shape is `internal/aptrepository`'s to declare. These
# two names are what the rig has to spell in a URL, and
# `tools/tests/test_native_install_rig.py` reads the Go source to fail when
# they drift.
REPOSITORY_PREFIX = "deb"
KEYRING_NAME = "internkim-archive-keyring.pgp"
BUILT_COMMAND_PATH = REPOSITORY_ROOT / ".artifacts" / "native-install-rig" / "internkim"


class RigFailure(Exception):
    pass


def run(arguments, **keywords):
    return subprocess.run(arguments, capture_output=True, text=True, **keywords)


def checked(arguments, **keywords):
    completed = run(arguments, **keywords)
    if completed.returncode != 0:
        raise RigFailure(f"{' '.join(arguments)} failed: {completed.stderr.strip() or completed.stdout.strip()}")
    return completed


def internkim_command():
    """The dev CLI, built from this checkout because the repository it renders
    has to be the one this branch publishes.

    `make build` writes ./internkim at the repository root; the rig builds its
    own copy instead so that running it never depends on a build step someone
    remembered, and never overwrites one someone is using.
    """
    if BUILT_COMMAND_PATH.exists():
        return BUILT_COMMAND_PATH
    BUILT_COMMAND_PATH.parent.mkdir(parents=True, exist_ok=True)
    checked(
        ["go", "build", "-o", str(BUILT_COMMAND_PATH), "./cmd/internkim"],
        cwd=str(REPOSITORY_ROOT),
    )
    return BUILT_COMMAND_PATH


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
    """An apt repository on this Mac, served to the guest over HTTP.

    The repository itself is built by `internkim release apt`, which is what
    publishes it to R2 for real. The rig substitutes the address it is served
    from and nothing else, so what a guest installs from here is what a
    customer installs from.
    """

    def __init__(self, directory):
        self.directory = Path(directory)
        self.repository_directory = self.directory / REPOSITORY_PREFIX
        self.staging_directory = self.directory.with_name(self.directory.name + "-packages")
        self.keyring_directory = Path(tempfile.mkdtemp(prefix="/tmp/ikrig-"))
        self.archive_key_path = self.keyring_directory / "archive-key.asc"
        self.public_keyring_path = self.repository_directory / KEYRING_NAME
        self.published = []
        self.served = []
        self.server = None
        self.port = None

    def generate_signing_key(self):
        """A throwaway key, named so that nothing mistakes it for the archive key.

        The real key's home is an open decision. The rig needs a key only so
        that apt has a signature to check, and it destroys this one on the way
        out.
        """
        if shutil.which("gpg") is None:
            raise RigFailure(
                "gpg is not on PATH; the rig signs its repository because an unsigned one would "
                "not exercise the Signed-By path the install is supposed to take"
            )
        self.keyring_directory.chmod(0o700)
        checked(
            [
                "gpg", "--homedir", str(self.keyring_directory), "--batch", "--yes", "--no-tty",
                "--pinentry-mode", "loopback", "--passphrase", "", "--quick-generate-key",
                "InternKim Install Rig TEST KEY <rig@invalid.internkim.test>", "default", "default", "never",
            ]
        )
        exported = checked(
            [
                "gpg", "--homedir", str(self.keyring_directory), "--batch", "--no-tty",
                "--pinentry-mode", "loopback", "--passphrase", "", "--armor", "--export-secret-keys",
            ]
        )
        self.archive_key_path.write_text(exported.stdout)
        self.archive_key_path.chmod(0o600)

    def publish(self, package_path, suite=SUITE):
        self.staging_directory.mkdir(parents=True, exist_ok=True)
        destination = self.staging_directory / Path(package_path).name
        shutil.copyfile(package_path, destination)
        self.published.append(destination)
        self.rebuild(suite)
        return destination

    def rebuild(self, suite=SUITE):
        """Render the repository with the command that publishes it for real."""
        checked(
            [
                str(internkim_command()), "release", "apt",
                "--suite", suite,
                "--package-directory", str(self.staging_directory),
                "--signing-key", str(self.archive_key_path),
                "--output", str(self.directory),
            ],
            cwd=str(REPOSITORY_ROOT),
        )

    def serve(self):
        """Serve the repository the way `workers/release-registry` serves it.

        The worker answers GET and HEAD from R2 and sets etag and
        content-length; it passes no conditional headers to R2 and so never
        answers 304. Dropping them here is what makes the rig's server
        wrong in the same way, rather than kinder than production.

        Every response is recorded, because what `apt-get update` costs
        against a server that cannot answer 304 is a number rather than an
        opinion.
        """
        import functools
        import http.server
        import threading

        served = self.served

        class QuietHandler(http.server.SimpleHTTPRequestHandler):
            def send_head(self):
                del self.headers["If-Modified-Since"]
                del self.headers["If-None-Match"]
                del self.headers["Range"]
                return super().send_head()

            def send_response(self, code, message=None):
                self.served_status = code
                self.served_length = 0
                return super().send_response(code, message)

            def send_header(self, keyword, value):
                if keyword.lower() == "content-length":
                    self.served_length = int(value)
                return super().send_header(keyword, value)

            def end_headers(self):
                # Recorded here rather than from log_request, which runs inside
                # send_response and so before Content-Length has been set.
                served.append(
                    {
                        "method": self.command,
                        "path": self.path,
                        "status": getattr(self, "served_status", 0),
                        "bytes": self.served_length if self.command == "GET" else 0,
                    }
                )
                return super().end_headers()

            def log_request(self, code="-", size=0):
                pass

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
        (f".{CONFFILE_PATH}", 0o644, b"# the stand-in's one conffile\n"),
        (
            "./lib/systemd/system/internkim-admind.service",
            0o644,
            stand_in_unit("internkim-admind", *ADMIN_GATEWAY_HEALTH).encode(),
        ),
        (
            "./lib/systemd/system/buzz-relay.service",
            0o644,
            stand_in_unit("buzz-relay", *MESSENGER_READINESS).encode(),
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
        ("./conffiles", 0o644, f"{CONFFILE_PATH}\n".encode()),
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
    """A unit shaped like the package's: it declines to start until a company exists.

    The real units carry ConditionPathExists over a file `internkim install`
    writes, so after an install every one of them is inactive. A stand-in
    without that condition would let the rig assert something no real package
    does.
    """
    return "\n".join(
        [
            "[Unit]",
            f"Description={name} (install rig stand-in)",
            "After=network.target",
            f"ConditionPathExists={COMPANY_CONDITION_PATH}",
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
  systemctl enable buzz-relay.service internkim-admind.service
  systemctl start buzz-relay.service internkim-admind.service || true
fi
exit 0
"""


STAND_IN_PRERM = """#!/bin/sh
set -e
if [ "$1" = remove ] || [ "$1" = deconfigure ]; then
  systemctl disable --now internkim-admind.service buzz-relay.service || true
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
