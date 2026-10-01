"""What differs between the Linux families the company host package installs on.

`tools/test-native-install-family` judges the same machine state on each of
them, and this is the one place that knows how a family spells the questions:
which image is a guest, how it gets a systemd, what its package database says
about an installed package, and how a package is removed. Nothing here decides
what a correct install looks like.
"""

from dataclasses import dataclass
from pathlib import Path

from native_install_rig import ARCHITECTURE, BOOTSTRAP_SCRIPT, DISTRIBUTIONS, PACKAGE_NAME, asset_name

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
        *(
            [
                "sed -i '1i Server = https://geo.mirror.pkgbuild.com/$repo/os/$arch' /etc/pacman.d/mirrorlist",
                "pacman -Sy --noconfirm archlinux-keyring",
                "pacman -Su --noconfirm --needed systemd curl",
            ]
            if ARCHITECTURE == "amd64"
            else ["pacman -Syu --noconfirm --needed systemd curl"]
        ),
        MAKE_RESOLVER_STATIC,
        "ln -sf /dev/null /etc/systemd/system/systemd-resolved.service",
        "exec /usr/lib/systemd/systemd",
    ]
)


@dataclass(frozen=True)
class Family:
    name: str
    image: str
    suffix: str
    bootstrap: str
    tools_command: str
    installed_status_command: str
    installed_answer: str
    version_command: str
    verify_command: str
    files_command: str
    remove_command: str
    package_manager: str
    unit_directories: tuple
    verify_ignored_lines: tuple = ("backup file",)
    postgresql_major_from_its_own_repository: str = ""

    @property
    def asset_name(self):
        return asset_name(self.suffix)

    def package_in(self, directory):
        path = Path(directory) / self.asset_name
        return path if path.is_file() else None


# Ubuntu 22.04's archive carries no pgvector, so install.sh takes PostgreSQL 15
# and its pgvector from PostgreSQL's own apt repository there.
JAMMY_POSTGRESQL_MAJOR = "15"

DEBIAN_TOOLS = "export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq iproute2 procps"


def debian_family(name):
    return Family(
        name=name,
        image=DISTRIBUTIONS[name],
        suffix=".deb",
        bootstrap=BOOTSTRAP_SCRIPT,
        tools_command=DEBIAN_TOOLS,
        installed_status_command=f"dpkg-query -W -f '${{Status}}' {PACKAGE_NAME}",
        installed_answer="install ok installed",
        version_command=f"dpkg-query -W -f '${{Version}}' {PACKAGE_NAME}",
        verify_command=f"dpkg --verify {PACKAGE_NAME}",
        files_command=f"dpkg-query -L {PACKAGE_NAME}",
        remove_command=f"export DEBIAN_FRONTEND=noninteractive; apt-get remove -y {PACKAGE_NAME}",
        package_manager="apt-get",
        unit_directories=("/lib/systemd/system", "/usr/lib/systemd/system"),
        postgresql_major_from_its_own_repository=JAMMY_POSTGRESQL_MAJOR if name == "ubuntu-22.04" else "",
    )


def rpm_family(name, image, bootstrap):
    return Family(
        name=name,
        image=image,
        suffix=".rpm",
        bootstrap=bootstrap,
        tools_command="dnf install -y -q iproute procps-ng",
        installed_status_command=f"rpm -q {PACKAGE_NAME} && echo {PRESENT_MARKER}",
        installed_answer=PRESENT_MARKER,
        version_command=f"rpm -q --qf '%{{VERSION}}' {PACKAGE_NAME}",
        verify_command=f"rpm -V {PACKAGE_NAME}",
        files_command=f"rpm -ql {PACKAGE_NAME}",
        remove_command=f"dnf remove -y {PACKAGE_NAME}",
        package_manager="dnf",
        unit_directories=("/usr/lib/systemd/system",),
    )


FAMILIES = {
    family.name: family
    for family in (
        *(debian_family(name) for name in DISTRIBUTIONS),
        rpm_family("fedora", "fedora:latest", FEDORA_BOOTSTRAP),
        rpm_family("rhel-10", "rockylinux/rockylinux:10", RHEL_BOOTSTRAP),
        Family(
            name="archlinux",
            image="lopsided/archlinux:latest",
            suffix=".pkg.tar.zst",
            bootstrap=ARCH_BOOTSTRAP,
            tools_command="pacman -S --noconfirm --needed iproute2 procps-ng",
            installed_status_command=f"pacman -Q {PACKAGE_NAME} && echo {PRESENT_MARKER}",
            installed_answer=PRESENT_MARKER,
            version_command=f"pacman -Q {PACKAGE_NAME} | cut -d ' ' -f 2 | sed 's/-[0-9]*$//'",
            verify_command=f"pacman -Qkk {PACKAGE_NAME}",
            files_command=f"pacman -Qlq {PACKAGE_NAME}",
            remove_command=f"pacman -R --noconfirm {PACKAGE_NAME}",
            package_manager="pacman",
            unit_directories=("/usr/lib/systemd/system",),
            verify_ignored_lines=("backup file", " total files, 0 altered files"),
        ),
    )
}
