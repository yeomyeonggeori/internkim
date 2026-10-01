"""The parts of the native-install rig that run on this Mac.

Serving a release the way GitHub serves one, and driving one disposable guest, arm64 or (INTERNKIM_RIG_ARCHITECTURE=amd64) amd64 under
Rosetta. The assertions that read the guest live in `tools/test-native-install`
and `tools/test-native-install-family`; everything here is what they need in
order to have a machine and something to install on it.
"""

import base64
import functools
import hashlib
import io
import json
import os
import posixpath
import re
import secrets
import shutil
import socket
import subprocess
import tarfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parent.parent
KERNEL_IMAGE_PATH = REPOSITORY_ROOT / ".dependency" / "container-kernel" / "Image-6.1.68-kvm"
DISTRIBUTIONS = {
    "debian-13": "debian:trixie-slim",
    "ubuntu-22.04": "ubuntu:22.04",
    "ubuntu-24.04": "ubuntu:24.04",
}
DEFAULT_DISTRIBUTION = "debian-13"
ARCHITECTURE = os.environ.get("INTERNKIM_RIG_ARCHITECTURE", "arm64")
if ARCHITECTURE not in ("arm64", "amd64"):
    raise SystemExit(f"INTERNKIM_RIG_ARCHITECTURE is {ARCHITECTURE!r}; the packages are built for arm64 and amd64")
MACHINE_NAME = {"arm64": "aarch64", "amd64": "x86_64"}[ARCHITECTURE]
RUNTIME_KERNEL = "runtime"
ROSETTA_WRITABLE_EXECUTABLE_UNITS = ("systemd-journald", "redis-server")
ROSETTA_BOOTSTRAP = (
    "mkdir -p /etc/systemd/system && ln -sf /dev/null /etc/systemd/system/systemd-binfmt.service\n"
    + "".join(
        f"mkdir -p /etc/systemd/system/{unit}.service.d && "
        f"printf '[Service]\\nMemoryDenyWriteExecute=no\\n' > /etc/systemd/system/{unit}.service.d/rosetta.conf\n"
        for unit in ROSETTA_WRITABLE_EXECUTABLE_UNITS
    )
)
DOCUMENT_MODULES_THE_CONVERSION_IMPORTS = (
    "import plistlib, platform, xml.etree.ElementTree, "
    "anydoc, bs4, markdownify, pypdf, pypdfium2"
)
HOST_PYTHON_VERSION = "3.13.13"
DOCUMENT_ENVIRONMENT_PATHS = ("/opt/internkim/python", "/opt/internkim/document-venv")
HOST_PYTHON_PATH = DOCUMENT_ENVIRONMENT_PATHS[0] + "/bin/python3"
OFFICE_COMMAND_PATH = "/opt/internkim/skills/office/scripts/office"
CAPABILITYD_UNIT = "internkim-capabilityd.service"
PACKAGE_NAME = "internkim"
SHARE_PATH = "/srv/internkim-rig"
STATE_DIRECTORY = "/var/lib/internkim"
CONFIGURATION_DIRECTORY = "/etc/internkim"
# Where anything asks whether a service is up, and which of those answers
# carries the revision the running process reports. Only the admin gateway's
# does: blueclaw's health body names its database, its language model and its
# protocol identity and no revision at all, which is why an upgrade cannot be
# judged from port 8080. `tools/tests/test_native_install_rig.py` reads the Go
# constants and fails when these drift from them.
#
# `internal/runtime/blueclaw/blueclaw_contract.go`'s BuzzRelayHealthPort names
# a second port, 3001, and a naive reading of that name suggests it is the
# relay's "real" health check and 3000 (the bind address) is not. Read
# against the relay's own router (`.dependency/buzz-relay/src/crates/buzz-relay/
# src/router.rs`), both ports register the identical `readiness_handler`:
# same Postgres ping, same Redis ping, same community-deletion-fence catalog
# check. 3001 exists only because the compose file this package replaced
# polled a health-only router on 8081; nothing about it is a deeper check.
# Switching this probe to 3001 would therefore change nothing.
#
# That check was tested directly against the pinned buzz-relay build
# (`tools/prepare-buzz-relay`'s pinned revision) in the exact state this
# probe exists to catch: a bind-address /_readiness answering while the
# messenger cannot register a person or store a message. A `buzz` database
# with zero tables never gets that far — the relay either refuses to start
# (BUZZ_AUTO_MIGRATE unset, this build's default) or runs every migration to
# completion before it opens a port (BUZZ_AUTO_MIGRATE=1, what the packaged
# unit sets). Breaking an already-serving instance by dropping the tables a
# person or a message actually needs (`users`, `events`, `relay_members`) —
# even by dropping and recreating the whole database out from under a relay
# process that is never restarted — makes /_readiness answer 503 on both
# 3000 and 3001, because the deletion-fence validation covers the full set
# of community-scoped tables, not just the three it names. Neither port is
# the blind spot for a schema that cannot serve a message; whatever answered
# 200 in that state left no reproduction this probe could see.
MESSENGER_READINESS = (3000, "/_readiness")
AGENT_HEALTH = (8080, "/admin/api/health")
ADMIN_GATEWAY_HEALTH = (18080, "/admin/api/health")
READINESS_PROBES = (MESSENGER_READINESS, AGENT_HEALTH, ADMIN_GATEWAY_HEALTH)
REVISION_PROBE = ADMIN_GATEWAY_HEALTH

# The bridge every message a person sends passes through on its way to the
# messenger. It is not in READINESS_PROBES because nothing else in step 5 waits
# on it; the member round trip does, because a call that reaches it before it
# listens is answered `did not answer` and says nothing about the product.
MESSENGER_BRIDGE = (18090, "/healthz")

# The package's one configuration file dpkg protects, and the file every unit
# waits on. Section 5 of the plan makes runtime.json an optional override the
# package does not ship, so editing that would say nothing about whether dpkg
# preserved an operator's work.
CONFFILE_PATH = "/etc/internkim/company-host.env"
CONFFILE_EDIT = "# edited-by-the-rig"
COMPANY_CONDITION_PATH = "/var/lib/internkim/current/host.env"

# Where GitHub answers for a repository's latest release, and the list each
# release carries of its files' checksums.
RELEASE_DOWNLOAD_PATH = "releases/latest/download"
CHECKSUMS_NAME = "SHA256SUMS"
INSTALL_SCRIPT_PATH = REPOSITORY_ROOT / "web" / "static" / "install.sh"


class RigFailure(Exception):
    pass


def run(arguments, environment=None, **keywords):
    """`environment` adds to this process's environment rather than replacing it,
    which is what `monkeys run` does when it hands a command a secret."""
    if environment is not None:
        keywords["env"] = {**os.environ, **environment}
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

    A name bounded from both sides, as `postgresql (>= 14), postgresql (<< 18)`
    would be, names one package twice; apt is asked for it once.
    """
    names = []
    for clause in depends_field.replace("\n", " ").split(","):
        first = clause.split("|")[0].split()
        if first and first[0] not in names:
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


def asset_name(suffix):
    """The name `internkim release packages` gives this architecture's package of one format.

    install.sh asks a release for the same name; `tools/tests/test_native_install_rig.py`
    holds this to the Go declaration.
    """
    return f"{PACKAGE_NAME}-{ARCHITECTURE}{suffix}"


def write_release_directory(directory, package_path, suffix=".deb"):
    """A directory shaped like the one `internkim release packages` writes, holding one package."""
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    name = asset_name(suffix)
    shutil.copyfile(package_path, directory / name)
    checksum = hashlib.sha256((directory / name).read_bytes()).hexdigest()
    (directory / CHECKSUMS_NAME).write_text(f"{checksum}  {name}\n")
    return directory


class Release:
    """A GitHub release's download directory, served to the guest over HTTP.

    `internkim release packages` writes the directory a release is uploaded
    from: each package under its asset name and their SHA256SUMS. The rig serves
    that directory where GitHub serves releases/latest/download and puts
    install.sh beside it, so the guest runs the published line with one
    address substituted and nothing else.
    """

    def __init__(self, directory):
        self.directory = Path(directory)
        self.download_directory = self.directory / RELEASE_DOWNLOAD_PATH
        self.server = None
        self.port = None

    def publish(self, release_directory):
        """Make the packages `release_directory`'s SHA256SUMS lists the latest release.

        The packages are linked rather than copied, because a release is six
        files of about 300 MB; `replace` swaps a link for a file of its own, so
        a tampering never writes through to the build.
        """
        release_directory = Path(release_directory).resolve()
        shutil.rmtree(self.download_directory, ignore_errors=True)
        self.download_directory.mkdir(parents=True)
        listed = (release_directory / CHECKSUMS_NAME).read_text()
        for line in listed.splitlines():
            name = line.split()[1]
            (self.download_directory / name).symlink_to(release_directory / name)
        (self.download_directory / CHECKSUMS_NAME).write_text(listed)
        shutil.copyfile(INSTALL_SCRIPT_PATH, self.directory / "install.sh")

    def replace(self, name, contents):
        """Serve `contents` as `name` and return what puts the published file back."""
        path = self.download_directory / name
        target = os.readlink(path) if path.is_symlink() else None
        original = None if target else path.read_bytes()
        path.unlink()
        path.write_bytes(contents)

        def restore():
            path.unlink()
            if target:
                path.symlink_to(target)
            else:
                path.write_bytes(original)

        return restore

    def published(self, name):
        return (self.download_directory / name).read_bytes()

    def download_url(self, host):
        return f"http://{host}:{self.port}/{RELEASE_DOWNLOAD_PATH}"

    def install_line(self, host):
        """The published one line, with only the release address pointed at this Mac."""
        return (
            f"set -eu\nexport INTERNKIM_INSTALL_RELEASE_URL={self.download_url(host)}\n"
            f"curl -fsSL http://{host}:{self.port}/install.sh | sh -s -- host\n"
        )

    def serve(self):
        import http.server
        import threading

        class QuietHandler(http.server.SimpleHTTPRequestHandler):
            def log_message(self, format, *arguments):
                pass

        handler = functools.partial(QuietHandler, directory=str(self.directory))
        self.server = http.server.ThreadingHTTPServer(("0.0.0.0", 0), handler)
        self.port = self.server.server_address[1]
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        return self.port

    def stop(self):
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
            "Depends: ca-certificates",
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
    echo "Delete it with: sudo rm -rf {STATE_DIRECTORY}"
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
        # The Debian container image ships a policy-rc.d that denies every service
        # action, so that building an image starts no daemon. A machine has no such
        # file, and leaving it here makes `deb-systemd-invoke stop` in the package's
        # prerm quietly do nothing while the rig reads the result as the package's
        # own behaviour.
        "rm -f /usr/sbin/policy-rc.d",
        "ln -sf /dev/null /etc/systemd/system/systemd-resolved.service",
        "exec /lib/systemd/systemd",
    ]
)

PRESEEDED_PACKAGES = ("systemd", "systemd-sysv", "curl", "ca-certificates")


class Machine:
    """One disposable guest of a Debian or Ubuntu release, created and destroyed by this rig alone."""

    def __init__(
        self,
        name,
        share_directory,
        container_binary="container",
        kernel_image_path=KERNEL_IMAGE_PATH,
        distribution=DEFAULT_DISTRIBUTION,
        image=None,
        bootstrap_script=BOOTSTRAP_SCRIPT,
    ):
        self.name = name
        self.image = image or DISTRIBUTIONS[distribution]
        self.bootstrap_script = bootstrap_script
        self.share_directory = Path(share_directory)
        self.container_binary = container_binary
        self.uses_runtime_kernel = str(kernel_image_path) == RUNTIME_KERNEL
        self.kernel_image_path = Path(kernel_image_path)
        self.created = False

    def preflight(self):
        if shutil.which(self.container_binary) is None:
            raise RigFailure("the `container` CLI is not on PATH; this rig uses the same runtime dev plane does")
        if not self.uses_runtime_kernel and not self.kernel_image_path.exists():
            raise RigFailure(f"{self.kernel_image_path} is missing; run tools/prepare-container-kernel")
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

    # The smallest appliance is the 4 GB CM5, so the guest gets what that box has:
    # step 5 has PostgreSQL, Redis, an S3 server, the messenger and the agent resident
    # at once, and a rig with more memory than the product would not be measuring it.
    def create(self, cpu_count=4, memory_mebibytes=4096):
        checked(
            [
                self.container_binary, "create",
                "--name", self.name,
                "--cpus", str(cpu_count),
                "--memory", f"{memory_mebibytes}M",
                "--tmpfs", "/run",
                "--tmpfs", "/run/lock",
                *([] if self.uses_runtime_kernel else ["--kernel", str(self.kernel_image_path)]),
                "--platform", "linux/" + ARCHITECTURE,
                "--volume", f"{self.share_directory}:{SHARE_PATH}",
                self.image,
                "sh", "-c", (ROSETTA_BOOTSTRAP if ARCHITECTURE == "amd64" else "") + self.bootstrap_script,
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
        try:
            return subprocess.run(
                [self.container_binary, "exec", self.name, "bash", f"{SHARE_PATH}/scripts/{script_path.name}"],
                capture_output=True,
                text=True,
                timeout=timeout_seconds,
            )
        except subprocess.TimeoutExpired as expired:
            # A raw TimeoutExpired escapes the driver's own handling and leaves
            # the guest and its several gigabytes behind. A RigFailure is what
            # the driver knows how to clean up after.
            raise RigFailure(
                f"the guest did not answer within {timeout_seconds}s: {script.strip().splitlines()[0]}"
            ) from expired

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


# ------------------------------------------------------- what a person does next

CONNECTION_FILE_NAME = "internkim-host.json"
MESSENGER_DATABASE_PATH = f"{STATE_DIRECTORY}/current/secrets/buzz-database.env"
MODEL_KEY_FILE_NAME = "rig-model-key"

# The guest is given a company whose agent never reaches a model provider. What
# any rig here judges is whether a message crosses the gateway and reaches the
# company's store, which no model is asked about; a key that could buy tokens
# has no business in a disposable guest.
RIG_MODEL_KEY = "sk-or-v1-this-rig-never-reaches-a-model-provider"


def capabilityd_argument(machine, flag):
    """What the installed capabilityd unit passes for one flag, read from systemd rather than from here."""
    shown = machine.shell(f"systemctl show -p ExecStart --value {CAPABILITYD_UNIT}").stdout
    words = shown.split("argv[]=", 1)[-1].split(";", 1)[0].split()
    return words[words.index(flag) + 1] if flag in words[:-1] else ""


def a_docx_saying(text):
    parts = {
        "[Content_Types].xml": (
            '<?xml version="1.0" encoding="UTF-8"?>'
            '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
            '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
            '<Default Extension="xml" ContentType="application/xml"/>'
            '<Override PartName="/word/document.xml" '
            'ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>'
            "</Types>"
        ),
        "_rels/.rels": (
            '<?xml version="1.0" encoding="UTF-8"?>'
            '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            '<Relationship Id="rId1" '
            'Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" '
            'Target="word/document.xml"/>'
            "</Relationships>"
        ),
        "word/document.xml": (
            '<?xml version="1.0" encoding="UTF-8"?>'
            '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
            f"<w:body><w:p><w:r><w:t>{text}</w:t></w:r></w:p></w:body></w:document>"
        ),
    }
    held = io.BytesIO()
    with zipfile.ZipFile(held, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, contents in parts.items():
            archive.writestr(name, contents)
    return held.getvalue()


def a_pdf_saying(text):
    stream = f"BT /F1 18 Tf 72 720 Td ({text}) Tj ET".encode()
    objects = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R "
        b"/Resources << /Font << /F1 5 0 R >> >> >>",
        b"<< /Length " + str(len(stream)).encode() + b" >>\nstream\n" + stream + b"\nendstream",
        b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    ]
    document = b"%PDF-1.4\n"
    offsets = []
    for number, body in enumerate(objects, start=1):
        offsets.append(len(document))
        document += f"{number} 0 obj\n".encode() + body + b"\nendobj\n"
    table = len(document)
    document += f"xref\n0 {len(objects) + 1}\n0000000000 65535 f \n".encode()
    document += b"".join(f"{offset:010d} 00000 n \n".encode() for offset in offsets)
    document += f"trailer\n<< /Size {len(objects) + 1} /Root 1 0 R >>\nstartxref\n{table}\n%%EOF\n".encode()
    return document


def read_through_capabilityd(machine, file_name, content):
    """Ask the running capabilityd to read a file the way blueclaw does: the file carried, the tool on its socket."""
    workspace_path = f"/workspace/{file_name}"
    request = {
        "toolName": "document_read",
        "input": {"path": workspace_path},
        "transport": {
            "workspaceFile": {
                "workspacePath": workspace_path,
                "filename": file_name,
                "contentBase64": base64.b64encode(content).decode(),
            }
        },
    }
    request_name = f"document-read-{file_name}.json"
    (machine.share_directory / request_name).write_text(json.dumps(request))
    socket_path = capabilityd_argument(machine, "--socket")
    return machine.shell(
        f"curl -sS --max-time 120 --unix-socket {socket_path} -H 'Content-Type: application/json' "
        f"--data-binary @{SHARE_PATH}/{request_name} http://capabilityd/v1/tools/document_read/invoke",
        timeout_seconds=180,
    )


def install_the_company(machine, connection):
    """Everything a person does between downloading the file and a running box."""
    share = machine.share_directory
    (share / CONNECTION_FILE_NAME).write_text(json.dumps(connection, indent=2))
    (share / MODEL_KEY_FILE_NAME).write_text(RIG_MODEL_KEY + "\n")
    return machine.shell(
        f"set -eu\n"
        f"{PACKAGE_NAME} install {SHARE_PATH}/{CONNECTION_FILE_NAME}"
        f" --model-key-file {SHARE_PATH}/{MODEL_KEY_FILE_NAME}\n",
        timeout_seconds=1800,
    )


def build_identity_reported(machine):
    """What the one endpoint that carries a revision says it is running.

    An upgrade that replaces a binary without restarting the unit leaves the
    old process serving, and dpkg's version and the file's mtime both move
    anyway. This is the process's own answer.
    """
    port, path = REVISION_PROBE
    reported = machine.shell(f"curl -s --max-time 10 http://127.0.0.1:{port}{path}")
    found = {}
    for name in ("admindBuildID", "gitRevision"):
        match = re.search(r'"' + name + r'"\s*:\s*"([^"]*)"', reported.stdout)
        found[name] = match.group(1) if match else ""
    return found


def installed_units(machine):
    """The units dpkg says this package put on the machine."""
    listed = machine.shell(
        f"dpkg-query -L {PACKAGE_NAME} 2>/dev/null"
        r" | grep -E '^/(lib|usr/lib)/systemd/system/.*\.service$' | xargs -r -n1 basename | sort -u"
    )
    return [name for name in listed.stdout.split() if name]


# ------------------------------------------------------------------- the plane

CENTRAL_TEST_UTILITIES_PATH = REPOSITORY_ROOT / "web" / "tests" / "e2e" / "central-test-utils.ts"
GATEWAY_WORKER_PATH = REPOSITORY_ROOT / "workers" / "connection-gateway"
COMPANY_COMPUTER_AGENT_NAME = "company-computer"
MEMBER_SESSION_SCRIPT = REPOSITORY_ROOT / "tools" / "native-install-member-session.ts"
MESSENGER_CREDENTIAL_KIND_SCHEMA = (
    REPOSITORY_ROOT
    / "pkg"
    / "capabilityprotocol"
    / "generated"
    / "json-schema"
    / "messenger-identity-credential-kind.schema.json"
)
HOST_SETUP_PATH = "/api/company/host-setup"
SIGN_IN_PATH = "/auth/v1/token?grant_type=password"


def messenger_credential_kind():
    """What the record calls the credential the box derives for each member.

    Read from the protocol's own schema rather than spelled here, because chatd
    and admind both take it from there and a third spelling would be the one
    that drifts.
    """
    schema = json.loads(MESSENGER_CREDENTIAL_KIND_SCHEMA.read_text())
    return schema["enum"][0]


def seeded_administrator():
    """The fixture admin, read from the file the browser suite signs in with.

    Repeating the address here would be a second copy of a credential that has to
    match what the development seed writes, and those two drifting apart is a
    refusal nobody can read.
    """
    document = CENTRAL_TEST_UTILITIES_PATH.read_text()
    fields = {}
    for name in ("member1Email", "seedPassword"):
        found = re.search(r"export const " + name + r" = '([^']+)'", document)
        if not found:
            raise RigFailure(f"{CENTRAL_TEST_UTILITIES_PATH} no longer exports {name}")
        fields[name] = found.group(1)
    return fields["member1Email"], fields["seedPassword"]


def free_port():
    with socket.socket() as held:
        held.bind(("127.0.0.1", 0))
        return held.getsockname()[1]


def local_plane_settings():
    completed = run(["supabase", "status", "--env", "--output-format", "text"], cwd=REPOSITORY_ROOT)
    if completed.returncode != 0:
        raise RigFailure(
            "this Mac is running no local plane, so there is no company to install. "
            "Start the stack and seed it, then run the rig again"
        )
    settings = {}
    for line in completed.stdout.splitlines():
        name, separator, value = line.partition("=")
        if separator:
            settings[name.strip()] = value.strip().strip("\"'")
    for required in ("API_URL", "PUBLISHABLE_KEY", "SECRET_KEY"):
        if not settings.get(required):
            raise RigFailure(f"the local plane named no {required}")
    return settings


SIGNING_KEY_SCRIPT = REPOSITORY_ROOT / "web" / "scripts" / "local-plane-signing-key.sh"


def local_plane_signing_key():
    """The key the app signs the host's token with, from the one definition of it."""
    completed = run(
        ["sh", "-c", '. "$1" && local_plane_signing_key "$2"', "sh", str(SIGNING_KEY_SCRIPT), str(REPOSITORY_ROOT)],
        cwd=REPOSITORY_ROOT,
    )
    if completed.returncode != 0 or not completed.stdout.strip():
        raise RigFailure(
            "the local plane published no signing key, so the app cannot mint a token the "
            f"gateway verifies: {completed.stderr.strip() or 'nothing was said'}"
        )
    return completed.stdout.strip()


def answered(url, headers=None, body=None, method="GET", timeout_seconds=10):
    request = urllib.request.Request(url, method=method, data=body)
    for name, value in (headers or {}).items():
        request.add_header(name, value)
    try:
        with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as refusal:
        return refusal.code, refusal.read()
    except (urllib.error.URLError, TimeoutError, ConnectionError):
        return 0, b""


class CompanyPlane:
    """The company's half of step 5: the record, the app and the gateway.

    All three run on this Mac and answer on every interface, because the guest
    reaches them at the address its own default route names. That address is the
    only substitution: the company, the member who signs in, the agent key and
    the connection document are the ones the app really issues.

    The record's address is the one thing written twice. Every token the gateway
    verifies has to carry the issuer the gateway was configured with, and GoTrue
    stamps the address it was started on rather than the one the caller reached
    it at. So the app and the gateway are both told that address, and the
    connection document the guest is handed carries one the guest can route to.
    """

    def __init__(self, working_directory, guest_reachable_address):
        self.working_directory = Path(working_directory)
        self.working_directory.mkdir(parents=True, exist_ok=True)
        self.address = guest_reachable_address
        self.settings = local_plane_settings()
        self.signing_key = local_plane_signing_key()
        self.administrator_email, self.administrator_password = seeded_administrator()
        # The app holds this to make its own calls to a company, and no
        # assertion here uses it: a call carrying it reaches the company as the
        # plane rather than as a member, which is not the wire a person is on.
        self.admin_token = secrets.token_hex(16)
        self.gateway_port = free_port()
        self.application_port = free_port()
        self.processes = []

    @property
    def gateway_url(self):
        return f"ws://{self.address}:{self.gateway_port}"

    @property
    def gateway_http_url(self):
        return f"http://127.0.0.1:{self.gateway_port}"

    @property
    def application_url(self):
        return f"http://{self.address}:{self.application_port}"

    @property
    def project_url(self):
        held = urllib.parse.urlsplit(self.settings["API_URL"])
        return f"{held.scheme}://{self.address}:{held.port}"

    def start(self):
        self.start_gateway()
        self.start_application()

    def start_gateway(self):
        configuration = json.loads((GATEWAY_WORKER_PATH / "wrangler.jsonc").read_text())
        configuration["main"] = str(GATEWAY_WORKER_PATH / "src" / "index.ts")
        # The gateway checks a token's issuer against this address, and the issuer
        # GoTrue stamps is the one it was configured with rather than the one the
        # caller reached it on. So the gateway is told that address and the guest
        # is told one it can route to.
        configuration["vars"] = {
            "SUPABASE_URL": self.settings["API_URL"],
            "SUPABASE_PUBLISHABLE_KEY": self.settings["PUBLISHABLE_KEY"],
            "GATEWAY_ADMIN_TOKEN": self.admin_token,
        }
        configuration_path = self.working_directory / "gateway.jsonc"
        configuration_path.write_text(json.dumps(configuration, indent=2))
        self.spawn(
            "the connection gateway",
            [
                "bunx", "wrangler", "dev", "--local", "--ip", "0.0.0.0",
                "--port", str(self.gateway_port), "--inspector-port", "0",
                "--persist-to", str(self.working_directory / "gateway-state"),
                "--config", str(configuration_path),
            ],
            GATEWAY_WORKER_PATH,
            {},
        )
        self.wait_until_answered(f"{self.gateway_http_url}/missing", 404, "the connection gateway")

    def start_application(self):
        self.spawn(
            "the company app",
            [
                "bunx", "vite", "dev", "--host", "0.0.0.0",
                "--port", str(self.application_port), "--strictPort",
            ],
            REPOSITORY_ROOT / "web",
            {
                "SUPABASE_URL": self.settings["API_URL"],
                "SUPABASE_PUBLISHABLE_KEY": self.settings["PUBLISHABLE_KEY"],
                "SUPABASE_SECRET_KEY": self.settings["SECRET_KEY"],
                "SUPABASE_JWT_SIGNING_KEY": self.signing_key,
                "GATEWAY_URL": self.gateway_url,
                "GATEWAY_ADMIN_TOKEN": self.admin_token,
            },
        )
        self.wait_until_answered(f"{self.application_url}/api/agent/company", None, "the company app")

    def spawn(self, name, command, working_directory, environment):
        log_path = self.working_directory / (name.replace(" ", "-") + ".log")
        handle = log_path.open("w")
        process = subprocess.Popen(
            command,
            cwd=str(working_directory),
            stdout=handle,
            stderr=handle,
            stdin=subprocess.DEVNULL,
            env={**os.environ, **environment},
        )
        self.processes.append((name, process, log_path, handle))

    def wait_until_answered(self, url, expected_status, name, timeout_seconds=180):
        deadline = time.monotonic() + timeout_seconds
        while time.monotonic() < deadline:
            status, _ = answered(url, timeout_seconds=3)
            if status and (expected_status is None or status == expected_status):
                return
            time.sleep(1)
        raise RigFailure(f"{name} never answered {url}\n{self.log_of(name)}")

    def log_of(self, name):
        for held, _, log_path, _ in self.processes:
            if held == name:
                return "\n".join(log_path.read_text().splitlines()[-30:])
        return ""

    def administrator_session(self):
        status, document = answered(
            self.settings["API_URL"] + SIGN_IN_PATH,
            {"apikey": self.settings["PUBLISHABLE_KEY"], "Content-Type": "application/json"},
            json.dumps({"email": self.administrator_email, "password": self.administrator_password}).encode(),
            "POST",
        )
        if status != 200:
            raise RigFailure(
                f"the seeded administrator {self.administrator_email} could not sign in ({status}); "
                "the local plane holds no company this rig can be given"
            )
        return json.loads(document)["access_token"]

    def connection_document(self):
        """What a person downloads from company setup, issued to this guest.

        With the record's address put back to one the guest can route to. The
        app signs the host's token with the issuer it was started on, which is
        the issuer the gateway checks, so it cannot be started on the guest's
        address; and the guest cannot reach the record at loopback.
        """
        status, document = answered(
            self.application_url + HOST_SETUP_PATH,
            {"Authorization": "Bearer " + self.administrator_session(), "Content-Type": "application/json"},
            json.dumps({"replaceExisting": True}).encode(),
            "POST",
            timeout_seconds=60,
        )
        if status != 200:
            raise RigFailure(
                f"company setup answered {status} instead of a connection document: {document[:300]!r}"
            )
        issued = json.loads(document)
        issued["centralPlane"]["projectURL"] = self.project_url
        return issued

    def member_session(self, company_id, message_text):
        """What a member does in a browser: sign in, open the wire, say something.

        Run in its own process because the browser's half of this wire is a
        websocket carrying the access token as a subprotocol, which is the shape
        `web/src/lib/host-bridge.ts` uses and the shape the gateway insists on.
        Nothing here holds the gateway's admin token or a server key.
        """
        completed = run(
            ["bun", "run", str(MEMBER_SESSION_SCRIPT)],
            cwd=REPOSITORY_ROOT,
            environment={
                "MEMBER_SESSION_PROJECT_URL": self.settings["API_URL"],
                "MEMBER_SESSION_PUBLISHABLE_KEY": self.settings["PUBLISHABLE_KEY"],
                "MEMBER_SESSION_GATEWAY_URL": self.gateway_url,
                "MEMBER_SESSION_COMPANY_ID": company_id,
                "MEMBER_SESSION_EMAIL": self.administrator_email,
                "MEMBER_SESSION_PASSWORD": self.administrator_password,
                "MEMBER_SESSION_MESSAGE": message_text,
            },
            timeout=300,
        )
        for line in reversed(completed.stdout.splitlines()):
            if line.startswith("{"):
                return json.loads(line)
        return {"refused": (completed.stderr.strip() or completed.stdout.strip() or "the member session said nothing")[-900:]}

    def member_identity(self, company_id):
        """The member row the seeded address belongs to on this company."""
        status, document = self.read_record(
            f"/rest/v1/member?select=id&company_id=eq.{company_id}"
            f"&email=eq.{urllib.parse.quote(self.administrator_email)}"
        )
        rows = json.loads(document) if status == 200 and document else []
        return rows[0]["id"] if rows else ""

    def wait_until_the_member_holds_a_messenger_credential(self, member_id, timeout_seconds=300):
        """admind derives each member's messenger key and records it here.

        The answer is the member's identity on the messenger, which is what the
        guest's own roster has to know them by. A member with no credential is
        refused by the relay with 409, so waiting for the record to hold one is
        waiting for a precondition rather than retrying a failure.
        """
        kind = messenger_credential_kind()
        deadline = time.monotonic() + timeout_seconds
        while True:
            status, document = self.read_record(
                f"/rest/v1/credential?select=external_id&kind=eq.{urllib.parse.quote(kind)}"
                f"&member_id=eq.{member_id}"
            )
            rows = json.loads(document) if status == 200 and document else []
            if rows and rows[0].get("external_id"):
                return rows[0]["external_id"]
            if time.monotonic() > deadline:
                return ""
            time.sleep(5)

    def read_record(self, path):
        return answered(
            self.settings["API_URL"] + path,
            {
                "apikey": self.settings["SECRET_KEY"],
                "Authorization": "Bearer " + self.settings["SECRET_KEY"],
            },
        )

    def forget_the_company_computer(self, company_id):
        """Take back the agent row the connection document wrote."""
        answered(
            self.settings["API_URL"]
            + "/rest/v1/agent?company_id=eq."
            + company_id
            + "&name=eq."
            + COMPANY_COMPUTER_AGENT_NAME,
            {
                "apikey": self.settings["SECRET_KEY"],
                "Authorization": "Bearer " + self.settings["SECRET_KEY"],
            },
            None,
            "DELETE",
        )

    def stop(self):
        for _, process, _, handle in reversed(self.processes):
            process.terminate()
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                process.kill()
            handle.close()
        self.processes = []
