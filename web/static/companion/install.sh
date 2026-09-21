#!/bin/sh
set -eu

# admind prints this address from internal/capabilities/protocol.go, so a device
# that has not been redeployed still sends people here.
script_url="${INTERNKIM_INSTALL_SCRIPT_URL:-https://intern.kim/install.sh}"
script="$(curl -fsSL "$script_url")"
exec sh -c "$script" install.sh companion
