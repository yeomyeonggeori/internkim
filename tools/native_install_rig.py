"""The parts of the native-install rig that run on this Mac.

Building a Debian package and an apt repository, and driving one disposable
arm64 Debian guest. The assertions that read the guest live in
`tools/test-native-install`; everything here is what they need in order to have
a machine and something to install on it.
"""

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
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parent.parent
KERNEL_IMAGE_PATH = REPOSITORY_ROOT / ".dependency" / "container-kernel" / "Image-6.1.68-kvm"
DEBIAN_SUITE = "trixie"
BASE_IMAGE = f"debian:{DEBIAN_SUITE}-slim"
ARCHITECTURE = "arm64"
SUITE = f"{DEBIAN_SUITE}-stable"
TESTING_SUITE = f"{DEBIAN_SUITE}-testing"
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

# The repository's own shape is `internal/aptrepository`'s to declare. These
# names are what the rig has to spell in a URL, and
# `tools/tests/test_native_install_rig.py` reads the Go source to fail when
# they drift.
REPOSITORY_PREFIX = "deb"
KEYRING_NAME = "internkim-archive-keyring.pgp"

# The name the vault holds the archive signing key under. The canonical copy is
# aptrepository.SigningKeyVariable in Go; TestTheRigNamesTheSameSigningKeyVariable
# reads that constant and fails if this drifts from it.
SIGNING_KEY_VARIABLE = "INTERNKIM_APT_SIGNING_KEY"
BUILT_COMMAND_PATH = REPOSITORY_ROOT / ".artifacts" / "native-install-rig" / "internkim"


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

    A name bounded from both sides, as `python3 (>= 3.13), python3 (<< 3.14)`
    is, names one package twice; apt is asked for it once.
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
                "--output", str(self.directory),
            ],
            cwd=str(REPOSITORY_ROOT),
            environment={
                SIGNING_KEY_VARIABLE: self.archive_key_path.read_text(),
            },
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
            # The real package bounds python3 to the minor version its document
            # interpreter was resolved against, and the stand-in carries the same
            # bound so that assertion has something to read. trixie's python3 is
            # inside it and bookworm's is not, which is the whole point of it.
            "Depends: python3 (>= 3.13), python3 (<< 3.14), ca-certificates",
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
    """One disposable arm64 Debian guest, created and destroyed by this rig alone."""

    def __init__(self, name, share_directory, container_binary="container", kernel_image_path=KERNEL_IMAGE_PATH):
        self.name = name
        self.share_directory = Path(share_directory)
        self.container_binary = container_binary
        self.kernel_image_path = Path(kernel_image_path)
        self.created = False

    def preflight(self):
        if shutil.which(self.container_binary) is None:
            raise RigFailure("the `container` CLI is not on PATH; this rig uses the same runtime the local fleet does")
        if not self.kernel_image_path.exists():
            raise RigFailure(f"{self.kernel_image_path} is missing; run `make prepare-container-kernel`")
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
                "--kernel", str(self.kernel_image_path),
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


# The suite the package was not built for. Its python3 is 3.11 where trixie's is
# 3.13, which is the difference the document interpreter's wheels cannot cross.
OTHER_SUITE_IMAGE = "debian:bookworm-slim"


def install_attempt_on_another_suite(package_path, container_binary="container"):
    """Ask a guest of another suite to install this package, and answer what apt said.

    A throwaway `container run` rather than a second Machine: nothing here needs an
    init, a kernel or a shared directory, because apt refuses before it unpacks
    anything.
    """
    package_path = Path(package_path)
    script = "\n".join([
        "set -u",
        "export DEBIAN_FRONTEND=noninteractive",
        "apt-get update -qq >/dev/null 2>&1",
        "apt-get install -y /package/" + package_path.name,
    ])
    completed = subprocess.run(
        [
            container_binary, "run", "--rm",
            "--platform", "linux/" + ARCHITECTURE,
            "--volume", str(package_path.parent) + ":/package",
            OTHER_SUITE_IMAGE, "sh", "-c", script,
        ],
        capture_output=True,
        text=True,
        timeout=900,
    )
    return completed

# ------------------------------------------------------- what a person does next

CONNECTION_FILE_NAME = "internkim-host.json"
MESSENGER_DATABASE_PATH = f"{STATE_DIRECTORY}/current/secrets/buzz-database.env"
MODEL_KEY_FILE_NAME = "rig-model-key"

# The guest is given a company whose agent never reaches a model provider. What
# any rig here judges is whether a message crosses the gateway and reaches the
# company's store, which no model is asked about; a key that could buy tokens
# has no business in a disposable guest.
RIG_MODEL_KEY = "sk-or-v1-this-rig-never-reaches-a-model-provider"


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
    completed = run(["supabase", "status", "-o", "env"], cwd=REPOSITORY_ROOT)
    if completed.returncode != 0:
        raise RigFailure(
            "this Mac is running no local plane, so there is no company to install. "
            "Start the stack and seed it, then run the rig again"
        )
    settings = {}
    for line in completed.stdout.splitlines():
        name, separator, value = line.partition("=")
        if separator:
            settings[name.strip()] = value.strip().strip('"')
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
