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

OPENSUSE_BOOTSTRAP = "\n".join(
    [
        "set -eu",
        "zypper --non-interactive install --no-recommends systemd curl gawk",
        MAKE_RESOLVER_STATIC,
        "ln -sf /dev/null /etc/systemd/system/systemd-resolved.service",
        "exec /usr/lib/systemd/systemd",
    ]
)

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

    def package_in(self, directory):
        found = sorted(
            path for path in Path(directory).iterdir() if fnmatch.fnmatch(path.name, self.package_pattern)
        )
        return found[-1] if found else None


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
        ),
        Family(
            name="opensuse-tumbleweed",
            image="opensuse/tumbleweed:latest",
            package_pattern=f"{PACKAGE_NAME}-*.aarch64.rpm",
            bootstrap=OPENSUSE_BOOTSTRAP,
            tools_command="zypper --non-interactive install --no-recommends iproute2 procps",
            installed_status_command=f"rpm -q {PACKAGE_NAME} && echo {PRESENT_MARKER}",
            installed_answer=PRESENT_MARKER,
            verify_command=f"rpm -V {PACKAGE_NAME}",
            files_command=f"rpm -ql {PACKAGE_NAME}",
            remove_command=f"zypper --non-interactive remove {PACKAGE_NAME}",
            package_manager="zypper",
            unit_directories=("/usr/lib/systemd/system",),
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
        ),
    )
}
