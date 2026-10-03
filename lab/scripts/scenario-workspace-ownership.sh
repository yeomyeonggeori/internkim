#!/usr/bin/env bash
set -euo pipefail

repository_path="$1"
workspace_path="$(mktemp -d)"
helper_path="$repository_path/.artifacts/workspace-ownership/blueclaw-posix-helper"
trap 'rm -rf "$workspace_path"' EXIT
chmod 0755 "$workspace_path"

getent group blueclaw >/dev/null || groupadd --system blueclaw
id blueclaw >/dev/null 2>&1 || useradd --system --no-create-home --gid blueclaw blueclaw

mkdir -p "$workspace_path/.blueclaw/config"
chown blueclaw:blueclaw "$workspace_path" "$workspace_path/.blueclaw"
cat > "$workspace_path/policy.json" <<'JSON'
{
  "people": [
    {"personID": "owner", "circles": ["team"]},
    {"personID": "reader", "circles": ["team"]},
    {"personID": "outsider", "circles": []}
  ],
  "circles": [{"circleID": "team"}]
}
JSON

sync_workspace() {
  "$helper_path" sync --workspace "$workspace_path" --policy "$workspace_path/policy.json"
}

sync_workspace
shared_project="$workspace_path/circles/team/sites/sample-site"
private_directory="$workspace_path/private/people/owner/drafts"
public_directory="$workspace_path/shared/public/sample-site"

runuser -u bc_person_owner -- bash -c '
  umask 007
  mkdir -p "$1"
  printf "shared content\n" > "$1/readme.txt"
  printf "#!/bin/sh\nprintf ready\n" > "$1/run.sh"
  chmod 0770 "$1/run.sh"
  umask 077
  mkdir -p "$2"
  printf "private content\n" > "$2/notes.txt"
  umask 022
  mkdir -p "$3"
  printf "public content\n" > "$3/index.html"
' bash "$shared_project" "$private_directory" "$public_directory"

snapshot_ownership() {
  stat -c '%u:%g:%a %n' \
    "$shared_project" "$shared_project/readme.txt" "$shared_project/run.sh" \
    "$private_directory" "$private_directory/notes.txt" \
    "$public_directory" "$public_directory/index.html"
}

assert_access() {
  test "$(runuser -u bc_person_reader -- cat "$shared_project/readme.txt")" = 'shared content'
  test "$(runuser -u bc_person_reader -- "$shared_project/run.sh")" = ready
  test "$(runuser -u bc_person_owner -- cat "$private_directory/notes.txt")" = 'private content'
  test "$(runuser -u bc_person_outsider -- cat "$public_directory/index.html")" = 'public content'
  if runuser -u bc_person_outsider -- cat "$shared_project/readme.txt" >/dev/null 2>&1; then
    echo 'non-member gained access to the team project' >&2
    return 1
  fi
  if runuser -u bc_person_reader -- cat "$private_directory/notes.txt" >/dev/null 2>&1; then
    echo 'another member gained access to private content' >&2
    return 1
  fi
}

assert_access
before_ownership="$(snapshot_ownership)"
for sync_number in 1 2; do
  sync_workspace
  after_ownership="$(snapshot_ownership)"
  if [ "$before_ownership" != "$after_ownership" ]; then
    printf 'ownership changed during workspace sync %s\nbefore:\n%s\nafter:\n%s\n' \
      "$sync_number" "$before_ownership" "$after_ownership" >&2
    exit 1
  fi
  assert_access
done
printf '%s\n' "$after_ownership"
echo 'workspace-ownership: preserved across repeated workspace syncs; member access and private boundaries verified'
