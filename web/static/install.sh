#!/bin/sh
set -eu

# The front door onto the package rather than a second installer. On a Debian
# machine `host` means apt: the keyring, a deb822 source naming it, and
# `apt-get install internkim`, so the box ends in the state it would have
# reached had the person typed those commands themselves and `apt upgrade` and
# `apt remove` work on it afterwards. On a Mac with Homebrew it means the tap
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
repository_url="${INTERNKIM_INSTALL_REPOSITORY_URL:-https://updates.intern.kim/deb}"
keyring_path="/usr/share/keyrings/internkim-archive-keyring.pgp"
keyring_url="${INTERNKIM_INSTALL_KEYRING_URL:-$repository_url/internkim-archive-keyring.pgp}"
apt_source_path="/etc/apt/sources.list.d/internkim.sources"
apt_component="main"
homebrew_tap="${INTERNKIM_INSTALL_HOMEBREW_TAP:-yeomyeonggeori/internkim}"

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

install_the_package() {
  require_administrator
  apt_suite="${INTERNKIM_INSTALL_SUITE:-}"
  # A suite name carries both how far a build is trusted and the Debian release
  # it was built against, because apt pins on the suite and then takes the
  # newest candidate inside it. The release half is this machine's own, so the
  # box follows the build made for it rather than one made for a Debian it is
  # not running.
  if [ -z "$apt_suite" ]; then
    debian_codename=""
    if [ -r /etc/os-release ]; then
      debian_codename="$(. /etc/os-release && printf '%s' "${VERSION_CODENAME:-}")"
    fi
    [ -n "$debian_codename" ] || stop \
"/etc/os-release names no VERSION_CODENAME, so this script cannot tell which
Debian release to ask apt for. Put the suite for this machine's release in
INTERNKIM_INSTALL_SUITE, which is trixie-stable on Debian 13, and run the same
command again."
    apt_suite="$debian_codename-stable"
  fi

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

  privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y "$package_name" || stop \
"apt-get install $package_name failed, and its own output above names what it
could not resolve. A dependency apt cannot find usually means this release of
Debian or Ubuntu does not carry it; send that line when you report this.
Undo what this script wrote with:
  sudo rm -f $apt_source_path $keyring_path"

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

  brew install "$package_name" || stop \
"brew install $package_name failed, and its own output above names what it could
not resolve.
Undo what this script did with:
  brew untap $homebrew_tap"

  echo
  echo "Installed $package_name. Every service stays idle until this box has a company."
  echo "Give it one with the connection file you downloaded from company setup:"
  echo "  sudo internkim install ~/Downloads/internkim-host.json"
}

if [ "$product" = host ] && command -v apt-get >/dev/null 2>&1; then
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
    echo "This machine has no apt, so the company host arrived as one binary rather than"
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
