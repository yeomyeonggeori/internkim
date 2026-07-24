#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
	echo "usage: build.sh <project-root>" >&2
	exit 2
fi

PROJECT_ROOT="$1"
APP_DIRECTORY="$PROJECT_ROOT/app"

if [ ! -f "$APP_DIRECTORY/scripts/build.ts" ]; then
	echo "Error: $APP_DIRECTORY/scripts/build.ts not found; run scaffold.sh first." >&2
	exit 1
fi

cd "$APP_DIRECTORY"
bun scripts/build.ts

QUALITY_PATH="../.internkim/build-quality.json"
if [ -f "$QUALITY_PATH" ]; then
	blocking_count="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("blockingIssueCount", 0))' "$QUALITY_PATH")"
	echo "Build finished: app/dist ready, $blocking_count blocking quality issue(s) in .internkim/build-quality.json."
else
	echo "Build finished: app/dist ready, but .internkim/build-quality.json is missing." >&2
fi
