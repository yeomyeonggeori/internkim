"""What differs between the Linux families the company host package installs on.

`tools/test-native-install-family` judges the same machine state on each of
them, and this is the one place that knows how a family spells the questions:
which image is a guest, how it gets a systemd, what its package database says
about an installed package, and how a package is removed. Nothing here decides
what a correct install looks like.
"""

import fnmatch
from dataclasses import dataclass
from pathlib import Path

from native_install_rig import BOOTSTRAP_SCRIPT, PACKAGE_NAME

PRESENT_MARKER = "internkim-is-present"

MAKE_RESOLVER_STATIC = (
    "if [ -L /etc/resolv.conf ]; then cp /etc/resolv.conf /etc/resolv.conf.static "
    "&& rm /etc/resolv.conf && mv /etc/resolv.conf.static /etc/resolv.conf; fi"
)

FEDORA_BOOTSTRAP = "\n".join(
    [
        "set -eu",
        "dnf install -y --setopt=install_weak_deps=False systemd curl",
        MAKE_RESOLVER_STATIC,
        "ln -sf /dev/null /etc/systemd/system/systemd-resolved.service",
        "exec /usr/lib/systemd/systemd",
    ]
)

RHEL_BOOTSTRAP = FEDORA_BOOTSTRAP.replace("dnf install -y", "dnf install -y --allowerasing")

ARCH_BOOTSTRAP = "\n".join(
    [
        "set -eu",
        "sed -i '/^\\[options\\]/a DisableSandbox' /etc/pacman.conf",
        "pacman -Syu --noconfirm --needed systemd curl",
        MAKE_RESOLVER_STATIC,
        "ln -sf /dev/null /etc/systemd/system/systemd-resolved.service",
        "exec /usr/lib/systemd/systemd",
    ]
)


@dataclass(frozen=True)
class Family:
    name: str
    image: str
    package_pattern: str
    bootstrap: str
    tools_command: str
    installed_status_command: str
    installed_answer: str
    verify_command: str
    files_command: str
    remove_command: str
    package_manager: str
    unit_directories: tuple
    verify_ignored_lines: tuple = ("backup file",)
    repository_format: str = ""
    policy_command: str = ""
    policy_answer: str = ""
    signature_command: str = ""
    signature_answer: str = ""
    reset_command: str = ""
    install_command: str = ""
    metadata_tampering: tuple = ()

    def package_in(self, directory):
        found = sorted(
            path for path in Path(directory).iterdir() if fnmatch.fnmatch(path.name, self.package_pattern)
        )
        return found[-1] if found else None


RPM_SIGNATURE_COMMAND = (
    f"rpm -q --qf '%{{RSAHEADER:pgpsig}}\\n' {PACKAGE_NAME}; rm -rf /tmp/downloaded; "
    f"dnf download --destdir /tmp/downloaded {PACKAGE_NAME} >/dev/null 2>&1; rpm -K /tmp/downloaded/*.rpm"
)
RPM_METADATA_TAMPERING = ((("rpm/stable/aarch64/repodata/repomd.xml", b"<revision>", b"<revision>9", False),),)
RPM_RESET = "dnf clean all"
RPM_INSTALL = f"dnf install -y {PACKAGE_NAME}"

DEBIAN_TOOLS = "export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq iproute2 procps"


def debian_family(name, image):
    return Family(
        name=name,
        image=image,
        package_pattern=f"{PACKAGE_NAME}_*_arm64.deb",
        bootstrap=BOOTSTRAP_SCRIPT,
        tools_command=DEBIAN_TOOLS,
        installed_status_command=f"dpkg-query -W -f '${{Status}}' {PACKAGE_NAME}",
        installed_answer="install ok installed",
        verify_command=f"dpkg --verify {PACKAGE_NAME}",
        files_command=f"dpkg-query -L {PACKAGE_NAME}",
        remove_command=f"export DEBIAN_FRONTEND=noninteractive; apt-get remove -y {PACKAGE_NAME}",
        package_manager="apt-get",
        unit_directories=("/lib/systemd/system", "/usr/lib/systemd/system"),
        repository_format="deb",
        policy_command="cat /etc/apt/sources.list.d/internkim.sources",
        policy_answer="Signed-By: /usr/share/keyrings/internkim-archive-keyring.pgp",
        signature_command=f"apt-cache policy {PACKAGE_NAME}",
        signature_answer="/deb stable/main",
        reset_command="apt-get clean; rm -rf /var/lib/apt/lists/*",
        install_command=f"export DEBIAN_FRONTEND=noninteractive; apt-get update && apt-get install -y {PACKAGE_NAME}",
        metadata_tampering=(
            (
                ("deb/dists/stable/InRelease", b"Origin: InternKim", b"Origin: InternKiM", False),
                ("deb/dists/stable/Release", b"Origin: InternKim", b"Origin: InternKiM", False),
            ),
        ),
    )


FAMILIES = {
    family.name: family
    for family in (
        debian_family("debian-13", "debian:trixie-slim"),
        debian_family("ubuntu-22.04", "ubuntu:22.04"),
        debian_family("ubuntu-24.04", "ubuntu:24.04"),
        Family(
            name="fedora",
            image="fedora:latest",
            package_pattern=f"{PACKAGE_NAME}-*.aarch64.rpm",
            bootstrap=FEDORA_BOOTSTRAP,
            tools_command="dnf install -y -q iproute procps-ng",
            installed_status_command=f"rpm -q {PACKAGE_NAME} && echo {PRESENT_MARKER}",
            installed_answer=PRESENT_MARKER,
            verify_command=f"rpm -V {PACKAGE_NAME}",
            files_command=f"rpm -ql {PACKAGE_NAME}",
            remove_command=f"dnf remove -y {PACKAGE_NAME}",
            package_manager="dnf",
            unit_directories=("/usr/lib/systemd/system",),
            repository_format="rpm",
            policy_command="grep -E '^(repo_)?gpgcheck=' /etc/yum.repos.d/internkim.repo",
            policy_answer="repo_gpgcheck=1",
            signature_command=RPM_SIGNATURE_COMMAND,
            signature_answer="digests signatures OK",
            reset_command=RPM_RESET,
            install_command=RPM_INSTALL,
            metadata_tampering=RPM_METADATA_TAMPERING,
        ),
        Family(
            name="rhel-10",
            image="rockylinux/rockylinux:10",
            package_pattern=f"{PACKAGE_NAME}-*.aarch64.rpm",
            bootstrap=RHEL_BOOTSTRAP,
            tools_command="dnf install -y -q iproute procps-ng",
            installed_status_command=f"rpm -q {PACKAGE_NAME} && echo {PRESENT_MARKER}",
            installed_answer=PRESENT_MARKER,
            verify_command=f"rpm -V {PACKAGE_NAME}",
            files_command=f"rpm -ql {PACKAGE_NAME}",
            remove_command=f"dnf remove -y {PACKAGE_NAME}",
            package_manager="dnf",
            unit_directories=("/usr/lib/systemd/system",),
            repository_format="rpm",
            policy_command="grep -E '^(repo_)?gpgcheck=' /etc/yum.repos.d/internkim.repo",
            policy_answer="repo_gpgcheck=1",
            signature_command=RPM_SIGNATURE_COMMAND,
            signature_answer="digests signatures OK",
            reset_command=RPM_RESET,
            install_command=RPM_INSTALL,
            metadata_tampering=RPM_METADATA_TAMPERING,
        ),
        Family(
            name="archlinux",
            image="lopsided/archlinux:latest",
            package_pattern=f"{PACKAGE_NAME}-*-aarch64.pkg.tar.zst",
            bootstrap=ARCH_BOOTSTRAP,
            tools_command="pacman -S --noconfirm --needed iproute2 procps-ng",
            installed_status_command=f"pacman -Q {PACKAGE_NAME} && echo {PRESENT_MARKER}",
            installed_answer=PRESENT_MARKER,
            verify_command=f"pacman -Qkk {PACKAGE_NAME}",
            files_command=f"pacman -Qlq {PACKAGE_NAME}",
            remove_command=f"pacman -R --noconfirm {PACKAGE_NAME}",
            package_manager="pacman",
            unit_directories=("/usr/lib/systemd/system",),
            verify_ignored_lines=("backup file", " total files, 0 altered files"),
            repository_format="archlinux",
            policy_command="grep -A1 '^\\[internkim\\]' /etc/pacman.conf",
            policy_answer="SigLevel = Required",
            signature_command=f"pacman -Qi {PACKAGE_NAME} | grep '^Validated By'",
            signature_answer="Signature",
            reset_command=f"rm -f /var/cache/pacman/pkg/{PACKAGE_NAME}-*",
            install_command=f"pacman -Syy --noconfirm && pacman -S --needed --noconfirm {PACKAGE_NAME}",
            metadata_tampering=((("arch/stable/aarch64/internkim.db", b"%NAME%\ninternkim", b"%NAME%\ninternkiM", True),),),
        ),
    )
}
