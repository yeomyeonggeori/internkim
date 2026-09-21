#!/bin/sh
set -eu

product="${1:-}"
case "$product" in
  companion|host) ;;
  *) echo "Usage: curl -fsSL https://intern.kim/install.sh | sh -s -- <companion|host>" >&2; exit 1 ;;
esac

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
  echo "Next, install your company server with the connection file you downloaded:"
  echo "  internkim-host install ~/Downloads/internkim-host.json"
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
