#!/bin/sh
set -eu

# The front door onto the package rather than a second installer. On a Linux
# machine `host` means its package manager, found by asking which of apt-get,
# dnf and pacman is there: the one package file for this machine is fetched
# from a GitHub Release of yeomyeonggeori/internkim, checked against the
# release's SHA256SUMS, and handed to that manager, which resolves its
# dependencies from the repositories the machine already trusts. Running the
# line again installs whatever the channel now points at, which is how the host
# is upgraded. `stable` is the latest release; `testing` is the newest one,
# prerelease or not, so a machine on it is never behind stable. On a Mac with
# Homebrew it means the tap and `brew install internkim`, so `brew upgrade` and
# `brew uninstall` work afterwards. A machine with neither is refused before
# anything changes.

product="${1:-}"
case "$product" in
  host) shift ;;
  *) echo "Usage: curl -fsSL https://intern.kim/install.sh | sh -s -- host [--channel stable|testing]" >&2; exit 1 ;;
esac

package_name="internkim"
release_repository="yeomyeonggeori/internkim"
channel="${INTERNKIM_INSTALL_CHANNEL:-stable}"
homebrew_tap="${INTERNKIM_INSTALL_HOMEBREW_TAP:-yeomyeonggeori/tap}"
os_release_path="${INTERNKIM_INSTALL_OS_RELEASE:-/etc/os-release}"
postgresql_key_url="${INTERNKIM_INSTALL_POSTGRESQL_KEY_URL:-https://www.postgresql.org/media/keys/ACCC4CF8.asc}"
postgresql_key_fingerprint="B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8"
postgresql_repository_url="https://apt.postgresql.org/pub/repos/apt"
postgresql_repository_origin="apt.postgresql.org"
postgresql_keyring_path="/usr/share/keyrings/internkim-postgresql-archive-keyring.asc"
postgresql_source_path="/etc/apt/sources.list.d/internkim-postgresql.sources"
postgresql_preferences_path="/etc/apt/preferences.d/internkim-postgresql.pref"
postgresql_major_from_its_repository="15"

stop() {
  echo "$1" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --channel) [ $# -ge 2 ] || stop "--channel needs stable or testing."; channel="$2"; shift 2 ;;
    --channel=*) channel="${1#--channel=}"; shift ;;
    *) stop "install.sh takes --channel stable|testing after the product, and was given $1." ;;
  esac
done
case "$channel" in
  stable|testing) ;;
  *) stop "The channel is stable or testing, and this install asked for $channel." ;;
esac

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

# The package manager this machine has, asked in a fixed order because some
# machines carry two: a Fedora with apt-get installed for a build is still a
# Fedora. Empty means none of the three the package is published for.
find_the_package_manager() {
  for candidate in apt-get dnf pacman; do
    if command -v "$candidate" >/dev/null 2>&1; then
      printf '%s' "$candidate"
      return 0
    fi
  done
}

package_architecture() {
  machine_architecture="$(uname -m)"
  case "$machine_architecture" in
    arm64|aarch64) printf 'arm64' ;;
    amd64|x86_64) printf 'amd64' ;;
    *) stop "The company host is published for arm64 and amd64, and this machine is $machine_architecture." ;;
  esac
}

package_suffix() {
  case "$package_manager" in
    apt-get) printf '.deb' ;;
    dnf) printf '.rpm' ;;
    pacman) printf '.pkg.tar.zst' ;;
  esac
}

# Where the release's files are. INTERNKIM_INSTALL_RELEASE_URL stands in for
# GitHub when a test serves a release of its own.
release_download_url() {
  if [ -n "${INTERNKIM_INSTALL_RELEASE_URL:-}" ]; then
    printf '%s' "$INTERNKIM_INSTALL_RELEASE_URL"
  elif [ "$channel" = stable ]; then
    printf '%s' "https://github.com/$release_repository/releases/latest/download"
  else
    newest_tag="$(newest_release_tag)" || exit 1
    printf '%s' "https://github.com/$release_repository/releases/download/$newest_tag"
  fi
}

# GitHub lists releases newest first, prereleases included, and one is all this
# asks for, so the response holds exactly one tag_name.
newest_release_tag() {
  releases_url="https://api.github.com/repos/$release_repository/releases?per_page=1"
  listed_tag="$(curl -fsSL "$releases_url" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')"
  [ -n "$listed_tag" ] || stop \
"Could not read the newest release from $releases_url.
Check that this machine can reach it, then run the same command again.
Nothing on this machine was changed."
  printf '%s' "$listed_tag"
}

fetch() {
  curl -fsSL "$1" -o "$2" || stop \
"Could not fetch $1.
Check that this machine can reach it, then run the same command again.
Nothing on this machine was changed."
}

# The checksum SHA256SUMS lists for one file, compared by whole name so that no
# other line can answer for it.
published_checksum() {
  while read -r listed_checksum listed_name; do
    if [ "$listed_name" = "$2" ]; then
      printf '%s' "$listed_checksum"
      return 0
    fi
  done < "$1"
}

verify_the_package() {
  expected_checksum="$(published_checksum "$work_dir/SHA256SUMS" "$asset_name")"
  [ -n "$expected_checksum" ] || stop \
"The release's SHA256SUMS lists no $asset_name, so it cannot be checked.
Nothing on this machine was changed."
  actual_checksum="$(sha256sum "$work_dir/$asset_name" | cut -d ' ' -f 1)"
  [ "$actual_checksum" = "$expected_checksum" ] || stop \
"$asset_name hashes to $actual_checksum, and the release's SHA256SUMS says
$expected_checksum. Nothing was installed, and nothing on this machine was changed."
}

install_the_package() {
  require_administrator
  architecture="$(package_architecture)" || exit 1
  asset_name="$package_name-$architecture$(package_suffix)"
  download_url="$(release_download_url)" || exit 1
  work_dir="$(mktemp -d)"
  trap 'rm -rf "$work_dir"' EXIT
  fetch "$download_url/SHA256SUMS" "$work_dir/SHA256SUMS"
  fetch "$download_url/$asset_name" "$work_dir/$asset_name"
  verify_the_package
  install_the_package_file "$work_dir/$asset_name" || stop \
"Installing $asset_name failed, and the package manager's own output above
names what it could not resolve. A dependency it cannot find usually means this
release of this distribution does not carry it."
  tell_what_to_do_next
}

# The manager's own command for a file, so the manager resolves the file's
# dependencies, which is the one thing a bare `dpkg -i` or `rpm -i` would not do.
install_the_package_file() {
  case "$package_manager" in
    apt-get)
      privileged apt-get update || stop "apt-get update failed, and its own output is above. Nothing on this machine was changed."
      make_pgvector_available_to_apt "$1"
      privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y "$1" ;;
    dnf) privileged dnf install -y "$1" ;;
    pacman) privileged pacman -U --needed --noconfirm "$1" ;;
  esac
}

# The package asks for pgvector by one name per PostgreSQL major, and apt is
# asked whether this machine's own sources offer any of them. Ubuntu 22.04's do
# not, so there PostgreSQL's own apt repository is added under a key checked
# against its published fingerprint, and pinned so that it supplies one major
# and the packages that major needs, and nothing else.
make_pgvector_available_to_apt() {
  accepted_pgvector_packages="$(pgvector_packages_the_package_accepts "$1")"
  [ -n "$accepted_pgvector_packages" ] || return 0
  apt_offers_one_of $accepted_pgvector_packages && return 0
  pinned_pgvector_package="postgresql-$postgresql_major_from_its_repository-pgvector"
  case " $(echo $accepted_pgvector_packages) " in
    *" $pinned_pgvector_package "*) ;;
    *) stop \
"This machine's apt sources offer none of the pgvector packages the package asks for:
  $(echo $accepted_pgvector_packages)
and $pinned_pgvector_package, the one PostgreSQL's own repository would be
pinned to, is not among them. Nothing on this machine was changed." ;;
  esac
  add_the_postgresql_repository
  apt_offers_one_of "$pinned_pgvector_package" || stop \
"Even with PostgreSQL's own repository added, apt offers no $pinned_pgvector_package.
Undo what this script wrote with:
  sudo rm -f $postgresql_source_path $postgresql_preferences_path $postgresql_keyring_path && sudo apt-get update"
}

pgvector_packages_the_package_accepts() {
  dpkg-deb --field "$1" Depends | tr ',|' '\n\n' | sed 's/(.*//; s/^ *//; s/ *$//' | grep -e '-pgvector$' || true
}

apt_offers_one_of() {
  for offered_name in "$@"; do
    candidate="$(apt-cache policy "$offered_name" 2>/dev/null | sed -n 's/^ *Candidate: //p')"
    if [ -n "$candidate" ] && [ "$candidate" != "(none)" ]; then
      return 0
    fi
  done
  return 1
}

add_the_postgresql_repository() {
  codename="$(sed -n 's/^VERSION_CODENAME=//p' "$os_release_path" 2>/dev/null | tr -d '"' | head -n 1)"
  [ -n "$codename" ] || stop \
"$os_release_path names no VERSION_CODENAME, so this script cannot tell which suite
of PostgreSQL's apt repository this machine would take pgvector from.
Nothing on this machine was changed."
  command -v gpg >/dev/null 2>&1 || privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends gpg || stop \
"gpg, which checks PostgreSQL's signing key, could not be installed, and apt's output is above.
Nothing else on this machine was changed."
  fetch "$postgresql_key_url" "$work_dir/postgresql-archive-keyring.asc"
  served_fingerprint="$(gpg --show-keys --with-colons "$work_dir/postgresql-archive-keyring.asc" 2>/dev/null | sed -n 's/^fpr:*\([0-9A-F]*\):$/\1/p' | head -n 1)"
  [ "$served_fingerprint" = "$postgresql_key_fingerprint" ] || stop \
"$postgresql_key_url served a key whose fingerprint is ${served_fingerprint:-unreadable},
and PostgreSQL publishes $postgresql_key_fingerprint for its apt repository.
Nothing was added, and nothing on this machine was changed."
  echo "This machine's apt sources carry no pgvector. Adding PostgreSQL's own apt repository"
  echo "($postgresql_repository_url, suite $codename-pgdg) for PostgreSQL $postgresql_major_from_its_repository and its pgvector only."
  printf '%s\n' \
    "Types: deb" \
    "URIs: $postgresql_repository_url" \
    "Suites: $codename-pgdg" \
    "Components: main" \
    "Signed-By: $postgresql_keyring_path" \
    > "$work_dir/postgresql.sources"
  printf '%s\n' \
    "Package: *" \
    "Pin: origin $postgresql_repository_origin" \
    "Pin-Priority: -1" \
    "" \
    "Package: postgresql-$postgresql_major_from_its_repository postgresql-client-$postgresql_major_from_its_repository postgresql-$postgresql_major_from_its_repository-pgvector postgresql-common postgresql-client-common libpq5" \
    "Pin: origin $postgresql_repository_origin" \
    "Pin-Priority: 500" \
    > "$work_dir/postgresql.pref"
  for directory in /usr/share/keyrings /etc/apt/sources.list.d /etc/apt/preferences.d; do
    privileged install -d -m 0755 "$directory" || stop "Could not make $directory. Nothing was added."
  done
  privileged install -m 0644 "$work_dir/postgresql-archive-keyring.asc" "$postgresql_keyring_path" &&
    privileged install -m 0644 "$work_dir/postgresql.pref" "$postgresql_preferences_path" &&
    privileged install -m 0644 "$work_dir/postgresql.sources" "$postgresql_source_path" || stop \
"Could not write PostgreSQL's repository into apt's configuration. Undo what this script wrote with:
  sudo rm -f $postgresql_source_path $postgresql_preferences_path $postgresql_keyring_path"
  privileged apt-get update || stop \
"apt-get update failed after PostgreSQL's own repository was added, and its output
is above. Undo what this script wrote with:
  sudo rm -f $postgresql_source_path $postgresql_preferences_path $postgresql_keyring_path && sudo apt-get update"
}

tell_what_to_do_next() {
  echo
  echo "Installed $package_name. Every service stays idle until this box has a company."
  echo "Give it one with the connection file you downloaded from company setup:"
  echo "  sudo internkim install ~/Downloads/internkim-host.json"
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

  tell_what_to_do_next
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

refuse_a_machine_with_no_supported_package_manager() {
  if [ "$(uname -s)" = Darwin ]; then
    stop \
"The company host on a Mac is installed through Homebrew, and this Mac has none.
Install it from https://brew.sh, then run the same command again.
Nothing on this machine was changed."
  fi
  stop \
"The company host is installed through apt, dnf or pacman, and this machine has
none of them. It is published for Debian, Ubuntu, Fedora, RHEL-compatible
distributions and Arch Linux.
Nothing on this machine was changed."
}

install_the_host() {
  package_manager="$(find_the_package_manager)"
  if [ -n "$package_manager" ]; then
    install_the_package
  elif command -v brew >/dev/null 2>&1; then
    install_through_homebrew
  else
    refuse_a_machine_with_no_supported_package_manager
  fi
}

install_the_host
