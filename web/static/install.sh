#!/bin/sh
set -eu

# The front door onto the package rather than a second installer. On a Linux
# machine `host` means its package manager, found by asking which of apt-get,
# dnf, pacman and zypper is there. With apt that is the keyring, a deb822
# source naming it, and `apt-get install internkim`, so the box ends in the
# state it would have reached had the person typed those commands themselves
# and `apt upgrade` and `apt remove` work on it afterwards. A package file the
# person already has is installed the same way with INTERNKIM_INSTALL_PACKAGE,
# which is a path or an address. On a Mac with Homebrew it means the tap
# and `brew install internkim`, for the same reason and with the same result:
# `brew upgrade` and `brew uninstall` work afterwards because nothing was put
# on the machine behind Homebrew's back. Everywhere else, and for `companion`
# always, the published binary is fetched against its checksum.

product="${1:-}"
case "$product" in
  companion|host) ;;
  *) echo "Usage: curl -fsSL https://intern.kim/install.sh | sh -s -- <companion|host>" >&2; exit 1 ;;
esac

package_name="internkim"
os_release_path="${INTERNKIM_INSTALL_OS_RELEASE:-/etc/os-release}"
package_file_source="${INTERNKIM_INSTALL_PACKAGE:-}"
package_file_checksum="${INTERNKIM_INSTALL_PACKAGE_SHA256:-}"
rpm_repository_url="${INTERNKIM_INSTALL_RPM_REPOSITORY_URL:-}"
rpm_repository_key_name="internkim-rpm-signing.asc"
pacman_repository_url="${INTERNKIM_INSTALL_PACMAN_REPOSITORY_URL:-}"
pacman_repository_key_name="internkim-pacman-signing.asc"
postgresql_repository_url="${INTERNKIM_INSTALL_POSTGRESQL_REPOSITORY_URL:-https://apt.postgresql.org/pub/repos/apt}"
postgresql_key_url="${INTERNKIM_INSTALL_POSTGRESQL_KEY_URL:-https://www.postgresql.org/media/keys/ACCC4CF8.asc}"
postgresql_key_fingerprint="B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8"
postgresql_keyring_path="/usr/share/keyrings/internkim-postgresql-archive-keyring.asc"
postgresql_source_path="/etc/apt/sources.list.d/internkim-postgresql.sources"
postgresql_preferences_path="/etc/apt/preferences.d/internkim-postgresql.pref"
repository_url="${INTERNKIM_INSTALL_REPOSITORY_URL:-https://updates.intern.kim/deb}"
keyring_path="/usr/share/keyrings/internkim-archive-keyring.pgp"
keyring_url="${INTERNKIM_INSTALL_KEYRING_URL:-$repository_url/internkim-archive-keyring.pgp}"
apt_source_path="/etc/apt/sources.list.d/internkim.sources"
apt_component="main"
homebrew_tap="${INTERNKIM_INSTALL_HOMEBREW_TAP:-yeomyeonggeori/tap}"

# BEGIN generated from internal/runtime/blueclaw/host_dependencies.go
memory_store_candidates_apt_get="postgresql-18-pgvector postgresql-17-pgvector postgresql-16-pgvector postgresql-15-pgvector postgresql-14-pgvector"
memory_store_candidates_dnf="pgvector"
memory_store_candidates_pacman="pgvector"
memory_store_candidates_zypper="postgresql18-pgvector postgresql17-pgvector postgresql16-pgvector postgresql15-pgvector postgresql14-pgvector"
# END generated

stop() {
  echo "$1" >&2
  exit 1
}

privileged() {
  if [ "$(id -u)" = 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

require_administrator() {
  if [ "$(id -u)" = 0 ]; then
    return 0
  fi
  command -v sudo >/dev/null 2>&1 || stop \
"Installing a package needs administrator rights and this machine has no sudo.
Run the same command as root:
  curl -fsSL https://intern.kim/install.sh | sh -s -- host"
  sudo -v || stop "sudo refused this account. Run the same command as root."
}

install_through_apt_repository() {
  apt_suite="${INTERNKIM_INSTALL_SUITE:-stable}"

  debian_architecture="$(dpkg --print-architecture)"
  case "$debian_architecture" in
    arm64|amd64) ;;
    *) stop "The company host is published for arm64 and amd64, and this machine is $debian_architecture." ;;
  esac

  package_work_dir="$(mktemp -d)"
  trap 'rm -rf "$package_work_dir"' EXIT

  curl -fsSL "$keyring_url" -o "$package_work_dir/keyring.pgp" || stop \
"Could not fetch the package signing key from $keyring_url.
Check that this machine can reach that address, then run the same command again.
Nothing on this machine was changed."
  [ -s "$package_work_dir/keyring.pgp" ] || stop \
"$keyring_url served an empty signing key, so apt would refuse every package it
signs. Nothing on this machine was changed; try again, and report it if it
happens twice."

  refuse_a_suite_the_repository_does_not_publish

  # /usr/share/keyrings, never apt-key: a key in the legacy keyring signs every
  # repository on the machine rather than only this one.
  privileged install -d -m 0755 /usr/share/keyrings
  privileged install -m 0644 "$package_work_dir/keyring.pgp" "$keyring_path"

  printf '%s\n' \
    "Types: deb" \
    "URIs: $repository_url" \
    "Suites: $apt_suite" \
    "Components: $apt_component" \
    "Architectures: $debian_architecture" \
    "Signed-By: $keyring_path" \
    > "$package_work_dir/internkim.sources"
  privileged install -d -m 0755 /etc/apt/sources.list.d
  privileged install -m 0644 "$package_work_dir/internkim.sources" "$apt_source_path"

  privileged apt-get update || stop \
"apt-get update failed, and its own output is above.
If the failure names $repository_url this machine cannot reach the package
repository; if it names another address, that source was already failing and
this install did not cause it.
Undo what this script wrote with:
  sudo rm -f $apt_source_path $keyring_path"

  make_the_memory_store_available

  privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y "$package_name" || stop \
"apt-get install $package_name failed, and its own output above names what it
could not resolve. A dependency apt cannot find usually means this release of
Debian or Ubuntu does not carry it; send that line when you report this.
Undo what this script wrote with:
  sudo rm -f $apt_source_path $keyring_path"
}

# The package manager this machine has, asked in a fixed order because some
# machines carry two: a Fedora with apt-get installed for a build is still a
# Fedora. Empty means none of the four the package is published for.
find_the_package_manager() {
  for candidate in apt-get dnf pacman zypper; do
    if command -v "$candidate" >/dev/null 2>&1; then
      printf '%s' "$candidate"
      return 0
    fi
  done
}

install_the_package() {
  require_administrator
  refuse_an_architecture_the_package_is_not_built_for
  if [ -n "$package_file_source" ]; then
    install_the_package_file
  else
    install_through_the_repository
  fi
  tell_what_to_do_next
}

refuse_an_architecture_the_package_is_not_built_for() {
  machine_architecture="$(uname -m)"
  case "$machine_architecture" in
    arm64|aarch64|amd64|x86_64) ;;
    *) stop "The company host is published for arm64 and amd64, and this machine is $machine_architecture." ;;
  esac
}

tell_what_to_do_next() {
  echo
  echo "Installed $package_name. Every service stays idle until this box has a company."
  echo "Give it one with the connection file you downloaded from company setup:"
  echo "  sudo internkim install ~/Downloads/internkim-host.json"
}

# The repository is how the package arrives by default. apt's is published;
# the other two are addressed by variables that are empty until their
# repository is, and an empty one says so rather than guessing.
install_through_the_repository() {
  case "$package_manager" in
    apt-get) install_through_apt_repository ;;
    dnf|zypper) install_through_rpm_repository ;;
    pacman) install_through_pacman_repository ;;
  esac
}

stop_for_an_unpublished_repository() {
  stop \
"$package_manager is on this machine, and no $1 repository is published for the
company host yet. Install the package file you have instead:
  INTERNKIM_INSTALL_PACKAGE=<path or address of the $1 file> \\
    curl -fsSL https://intern.kim/install.sh | sh -s -- host
Nothing on this machine was changed."
}

install_through_rpm_repository() {
  [ -n "$rpm_repository_url" ] || stop_for_an_unpublished_repository rpm
  package_work_dir="$(mktemp -d)"
  trap 'rm -rf "$package_work_dir"' EXIT
  rpm_key_url="$rpm_repository_url/$rpm_repository_key_name"
  printf '%s\n' \
    "[$package_name]" \
    "name=$package_name" \
    "baseurl=$rpm_repository_url" \
    "enabled=1" \
    "gpgcheck=1" \
    "repo_gpgcheck=1" \
    "gpgkey=$rpm_key_url" \
    > "$package_work_dir/$package_name.repo"
  case "$package_manager" in
    dnf)
      privileged install -m 0644 "$package_work_dir/$package_name.repo" "/etc/yum.repos.d/$package_name.repo"
      make_the_memory_store_available
      privileged dnf install -y "$package_name" || stop \
"dnf install $package_name failed, and its own output above names what it could
not resolve. Undo what this script wrote with:
  sudo rm -f /etc/yum.repos.d/$package_name.repo" ;;
    zypper)
      privileged zypper --non-interactive addrepo --gpgcheck --refresh "$rpm_repository_url" "$package_name"
      privileged zypper --non-interactive --gpg-auto-import-keys refresh "$package_name"
      make_the_memory_store_available
      privileged zypper --non-interactive install "$package_name" || stop \
"zypper install $package_name failed, and its own output above names what it
could not resolve. Undo what this script wrote with:
  sudo zypper removerepo $package_name" ;;
  esac
}

install_through_pacman_repository() {
  [ -n "$pacman_repository_url" ] || stop_for_an_unpublished_repository "Arch Linux"
  package_work_dir="$(mktemp -d)"
  trap 'rm -rf "$package_work_dir"' EXIT
  curl -fsSL "$pacman_repository_url/$pacman_repository_key_name" -o "$package_work_dir/key.asc" || stop \
"Could not fetch the package signing key from $pacman_repository_url/$pacman_repository_key_name.
Nothing on this machine was changed."
  pacman_key_fingerprint="$(gpg --show-keys --with-colons "$package_work_dir/key.asc" | sed -n 's/^fpr:::::::::\([0-9A-F]*\):$/\1/p' | head -n 1)"
  [ -n "$pacman_key_fingerprint" ] || stop "$pacman_repository_url served no signing key. Nothing on this machine was changed."
  privileged pacman-key --add "$package_work_dir/key.asc"
  privileged pacman-key --lsign-key "$pacman_key_fingerprint"
  printf '\n[%s]\nSigLevel = Required DatabaseOptional\nServer = %s/$arch\n' "$package_name" "$pacman_repository_url" > "$package_work_dir/pacman-source.conf"
  privileged sh -c "cat '$package_work_dir/pacman-source.conf' >> /etc/pacman.conf"
  privileged pacman -Sy --noconfirm
  make_the_memory_store_available
  privileged pacman -S --needed --noconfirm "$package_name" || stop \
"pacman -S $package_name failed, and its own output above names what it could
not resolve. The [$package_name] section this script appended to /etc/pacman.conf
is still there; remove it to undo this."
}

# A package file the person already has, by path or by address. The file is
# installed with the manager's own command for one, so the manager resolves its
# dependencies from the repositories the machine already trusts, which is the
# one thing a bare `dpkg -i` or `rpm -i` would not do.
install_the_package_file() {
  package_work_dir="$(mktemp -d)"
  trap 'rm -rf "$package_work_dir"' EXIT
  case "$package_manager" in
    apt-get) package_file_suffix=".deb" ;;
    dnf|zypper) package_file_suffix=".rpm" ;;
    pacman) package_file_suffix=".pkg.tar.zst" ;;
  esac
  case "$package_file_source" in
    http://*|https://*)
      package_file_path="$package_work_dir/$(basename "$package_file_source")"
      curl -fsSL "$package_file_source" -o "$package_file_path" || stop \
"Could not fetch $package_file_source. Nothing on this machine was changed." ;;
    *)
      [ -f "$package_file_source" ] || stop \
"$package_file_source is not a file. Nothing on this machine was changed."
      package_file_path="$(cd "$(dirname "$package_file_source")" && pwd)/$(basename "$package_file_source")" ;;
  esac
  case "$package_file_path" in
    *"$package_file_suffix") ;;
    *) stop \
"$package_manager installs $package_file_suffix files and $package_file_path is not one.
Nothing on this machine was changed." ;;
  esac
  verify_the_package_file_checksum "$package_file_path"
  if [ "$package_manager" = apt-get ]; then
    privileged apt-get update || stop "apt-get update failed, and its own output is above. Nothing on this machine was changed."
  fi
  make_the_memory_store_available
  case "$package_manager" in
    apt-get) privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y "$package_file_path" ;;
    dnf) privileged dnf install -y "$package_file_path" ;;
    zypper) privileged zypper --non-interactive --no-gpg-checks install "$package_file_path" ;;
    pacman) privileged pacman -U --needed --noconfirm "$package_file_path" ;;
  esac || stop \
"Installing $package_file_path failed, and the package manager's own output above
names what it could not resolve. A dependency it cannot find usually means this
release of this distribution does not carry it."
}

verify_the_package_file_checksum() {
  [ -n "$package_file_checksum" ] || return 0
  if command -v sha256sum >/dev/null 2>&1; then
    actual_package_checksum="$(sha256sum "$1" | cut -d ' ' -f 1)"
  else
    actual_package_checksum="$(shasum -a 256 "$1" | cut -d ' ' -f 1)"
  fi
  [ "$actual_package_checksum" = "$package_file_checksum" ] || stop \
"$1 hashes to $actual_package_checksum and INTERNKIM_INSTALL_PACKAGE_SHA256 says
$package_file_checksum. Nothing on this machine was changed."
}

# The memory store keeps its embeddings in pgvector, and a PostgreSQL without
# it is a memory that stores nothing and says nothing. The package names the
# pgvector it needs, and whether this machine's repositories carry any of
# those names is asked of the package manager, so a distribution that begins
# carrying it stops needing anything else without this script being edited.
# Where apt's do not, the PostgreSQL project's own repository is added, scoped:
# a key pinned by fingerprint that signs that repository alone, and a pin that
# keeps it from replacing a package the distribution already carries.
make_the_memory_store_available() {
  memory_store_candidates="$(memory_store_candidates_of "$package_manager")"
  if a_memory_store_candidate_is_available; then
    return 0
  fi
  [ "$package_manager" = apt-get ] || stop \
"None of the pgvector packages this host needs is in this machine's repositories:
  $memory_store_candidates
The memory store cannot keep what the agent learns without it. Nothing on this
machine was changed."
  add_the_postgresql_apt_repository
  a_memory_store_candidate_is_available || stop \
"Even with the PostgreSQL project's repository added, apt offers none of:
  $memory_store_candidates
Undo what this script wrote with:
  sudo rm -f $postgresql_source_path $postgresql_keyring_path $postgresql_preferences_path"
}

memory_store_candidates_of() {
  case "$1" in
    apt-get) printf '%s' "$memory_store_candidates_apt_get" ;;
    dnf) printf '%s' "$memory_store_candidates_dnf" ;;
    pacman) printf '%s' "$memory_store_candidates_pacman" ;;
    zypper) printf '%s' "$memory_store_candidates_zypper" ;;
  esac
}

a_memory_store_candidate_is_available() {
  for candidate_name in $memory_store_candidates; do
    case "$package_manager" in
      apt-get)
        offered="$(apt-cache policy "$candidate_name" 2>/dev/null | sed -n 's/^  Candidate: //p')"
        [ -n "$offered" ] && [ "$offered" != "(none)" ] && return 0 ;;
      dnf)
        [ -n "$(dnf -q repoquery "$candidate_name" 2>/dev/null)" ] && return 0 ;;
      pacman)
        pacman -Si "$candidate_name" >/dev/null 2>&1 && return 0 ;;
      zypper)
        zypper --non-interactive search --match-exact --type package "$candidate_name" 2>/dev/null | grep -q "| package" && return 0 ;;
    esac
  done
  return 1
}

add_the_postgresql_apt_repository() {
  postgresql_codename=""
  if [ -r "$os_release_path" ]; then
    postgresql_codename="$(sed -n 's/^VERSION_CODENAME=//p' "$os_release_path" | tr -d '"'"'"'"' | head -n 1)"
  fi
  [ -n "$postgresql_codename" ] || stop \
"$os_release_path names no VERSION_CODENAME, so this script cannot tell which
PostgreSQL repository suite this machine's release uses."
  command -v gpg >/dev/null 2>&1 || privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends gpg
  curl -fsSL "$postgresql_key_url" -o "$package_work_dir/postgresql-key.asc" || stop \
"Could not fetch the PostgreSQL project's signing key from $postgresql_key_url.
Nothing on this machine was changed."
  fetched_key_fingerprint="$(gpg --show-keys --with-colons "$package_work_dir/postgresql-key.asc" | sed -n 's/^fpr:::::::::\([0-9A-F]*\):$/\1/p' | head -n 1)"
  [ "$fetched_key_fingerprint" = "$postgresql_key_fingerprint" ] || stop \
"$postgresql_key_url served a key with fingerprint ${fetched_key_fingerprint:-none}, and this
script trusts only $postgresql_key_fingerprint for that repository.
Nothing on this machine was changed."
  privileged install -d -m 0755 /usr/share/keyrings
  privileged install -d -m 0755 /etc/apt/preferences.d
  privileged install -d -m 0755 /etc/apt/sources.list.d
  privileged install -m 0644 "$package_work_dir/postgresql-key.asc" "$postgresql_keyring_path"
  printf '%s\n' \
    "Types: deb" \
    "URIs: $postgresql_repository_url" \
    "Suites: $postgresql_codename-pgdg" \
    "Components: main" \
    "Signed-By: $postgresql_keyring_path" \
    > "$package_work_dir/postgresql.sources"
  privileged install -m 0644 "$package_work_dir/postgresql.sources" "$postgresql_source_path"
  printf '%s\n' \
    "Package: *" \
    "Pin: origin $(printf '%s' "$postgresql_repository_url" | sed 's#^[a-z]*://##; s#/.*##')" \
    "Pin-Priority: 100" \
    > "$package_work_dir/postgresql.pref"
  privileged install -m 0644 "$package_work_dir/postgresql.pref" "$postgresql_preferences_path"
  privileged apt-get update || stop \
"apt-get update failed after adding the PostgreSQL project's repository, and its
own output is above. Undo what this script wrote with:
  sudo rm -f $postgresql_source_path $postgresql_keyring_path $postgresql_preferences_path"
}

# A suite nobody published makes `apt-get update` fail on the whole source, and
# that failure reads as this machine or this address being wrong when neither
# is. The suite's own index answers it directly, and the answer has two shapes
# that want different sentences: a status is a suite this repository does not
# carry, and no status at all is the repository being out of reach. This runs
# after the signing key has already been fetched, so the repository has served
# this machine something by the time it is asked.
refuse_a_suite_the_repository_does_not_publish() {
  suite_index_url="$repository_url/dists/$apt_suite/InRelease"
  suite_probe_status=0
  suite_index_code="$(curl -sSL -o /dev/null -w '%{http_code}' "$suite_index_url")" || suite_probe_status=$?

  case "$suite_index_code" in
    2??) return 0 ;;
  esac

  if [ "$suite_probe_status" != 0 ] || [ "$suite_index_code" = 000 ]; then
    stop \
"Could not reach the repository to see whether $apt_suite is published, and
curl's own reason is above.
$suite_index_url
Check that this machine can reach it, then run the same command again.
Nothing on this machine was changed."
  fi

  stop \
"This repository publishes nothing at $apt_suite, which is the suite
this install asked for. If INTERNKIM_INSTALL_SUITE is set, check it against what
the repository publishes.
$suite_index_url answered $suite_index_code.
Nothing on this machine was changed."
}

# Homebrew is not run as root. It refuses to be, and the files it writes belong
# to the person who installed them; what needs root is the second line, and that
# is `internkim install`, which asks for it itself.
install_through_homebrew() {
  brew tap "$homebrew_tap" || stop \
"brew tap $homebrew_tap failed, and its own output is above.
Nothing on this machine was changed."

  trust_the_formula

  brew install "$package_name" || stop \
"brew install $package_name failed, and its own output above names what it could
not resolve.
Undo what this script did with:
  brew untrust --formula $homebrew_tap/$package_name && brew untap $homebrew_tap"

  echo
  echo "Installed $package_name. Every service stays idle until this box has a company."
  echo "Give it one with the connection file you downloaded from company setup:"
  echo "  sudo internkim install ~/Downloads/internkim-host.json"
}

# Homebrew 7 refuses to load a formula from a tap nobody has trusted, and the
# variable that turned that off is deprecated, so `brew trust` is the way. This
# trusts the one formula rather than the whole tap: a second formula published
# here is a second decision. Older Homebrew has no such command and no such
# gate, which is why this asks before it runs it.
trust_the_formula() {
  brew trust --help >/dev/null 2>&1 || return 0
  echo "Trusting $homebrew_tap/$package_name, which is what lets brew load a formula that is not its own."
  brew trust --formula "$homebrew_tap/$package_name" || stop \
"brew trust --formula $homebrew_tap/$package_name failed, and brew will not load
the formula until it succeeds.
Undo what this script did with:
  brew untap $homebrew_tap"
}

package_manager=""
if [ "$product" = host ]; then
  package_manager="$(find_the_package_manager)"
fi

if [ -n "$package_manager" ]; then
  install_the_package
  exit 0
fi

if [ "$product" = host ] && command -v brew >/dev/null 2>&1; then
  install_through_homebrew
  exit 0
fi

binary="internkim-$product"
release_url="${INTERNKIM_INSTALL_RELEASE_URL:-https://updates.intern.kim/$product/latest}"
bin_dir="${INTERNKIM_INSTALL_BIN_DIR:-$HOME/.local/bin}"
binary_path="$bin_dir/$binary"

operating_system="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$operating_system" in
  darwin|linux) ;;
  *) echo "$binary has no build for $operating_system" >&2; exit 1 ;;
esac

architecture="$(uname -m)"
case "$architecture" in
  arm64|aarch64) architecture=arm64 ;;
  x86_64|amd64) architecture=amd64 ;;
  *) echo "$binary has no build for $architecture" >&2; exit 1 ;;
esac

binary_name="$binary-$operating_system-$architecture"
download_dir="$(mktemp -d)"
trap 'rm -rf "$download_dir"' EXIT

curl -fsSL "$release_url/$binary_name" -o "$download_dir/$binary_name"
curl -fsSL "$release_url/SHA256SUMS" -o "$download_dir/SHA256SUMS"

expected_checksum="$(grep " $binary_name\$" "$download_dir/SHA256SUMS" | cut -d ' ' -f 1)"
if command -v sha256sum >/dev/null 2>&1; then
  actual_checksum="$(sha256sum "$download_dir/$binary_name" | cut -d ' ' -f 1)"
else
  actual_checksum="$(shasum -a 256 "$download_dir/$binary_name" | cut -d ' ' -f 1)"
fi
if [ -z "$expected_checksum" ] || [ "$expected_checksum" != "$actual_checksum" ]; then
  echo "$binary_name does not match the published checksum" >&2
  exit 1
fi

mkdir -p "$bin_dir"
install -m 0755 "$download_dir/$binary_name" "$binary_path"
echo "Installed $binary_path"

case ":$PATH:" in
  *":$bin_dir:"*) ;;
  *) echo "Add it to your PATH: export PATH=\"$bin_dir:\$PATH\"" ;;
esac

if [ "$product" = "host" ]; then
  echo
  if [ "$operating_system" = darwin ]; then
    echo "This Mac has no Homebrew, so the company host arrived as one binary rather than"
    echo "a package. Homebrew is where the database, the cache and the rest come from:"
    echo "  https://brew.sh"
    echo "Install it, run this line again, and the box ends registered with brew."
  else
    echo "This machine has none of apt, dnf, pacman or zypper, so the company host arrived as one binary rather than"
    echo "a package. Install the company server with the connection file you downloaded:"
  fi
  echo "  sudo $binary_path install ~/Downloads/internkim-host.json"
  exit 0
fi

if "$binary_path" service status 2>/dev/null | grep -q '^running'; then
  "$binary_path" service restart
  echo "Restarted the background service."
  exit 0
fi

echo
echo "Next, pair it with your company computer using the code Intern Kim gave you:"
echo "  internkim-companion pair --device-url https://<your company computer> --code <code>"
echo "Then keep it running:"
echo "  internkim-companion service install"
