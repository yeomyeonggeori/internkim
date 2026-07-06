#!/bin/bash
set -e
SCRIPT_DIRECTORY="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRESENTATION_BUILD_PATH="/workspace/skills/presentation/scripts/build.sh"
if [ ! -x "$PRESENTATION_BUILD_PATH" ]; then
  PRESENTATION_BUILD_PATH="${SCRIPT_DIRECTORY}/../../presentation/scripts/build.sh"
fi
exec "$PRESENTATION_BUILD_PATH" "$@"
