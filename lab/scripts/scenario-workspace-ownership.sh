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
circle_memory="$workspace_path/circles/team/.protected/memory.db"
private_directory="$workspace_path/private/people/owner/drafts"

runuser -u blueclaw -- bash -c 'printf "circle memory\n" > "$1"' bash "$circle_memory"
runuser -u bc_person_owner -- bash -c '
  umask 077
  mkdir -p "$1"
  printf "private content\n" > "$1/notes.txt"
' bash "$private_directory"

snapshot_ownership() {
  stat -c '%u:%g:%a %n' \
    "$workspace_path/circles/team" "$circle_memory" \
    "$private_directory" "$private_directory/notes.txt"
}

assert_access() {
  test "$(runuser -u bc_person_reader -- cat "$circle_memory")" = 'circle memory'
  test "$(runuser -u bc_person_owner -- cat "$private_directory/notes.txt")" = 'private content'
  if runuser -u bc_person_owner -- touch "$workspace_path/circles/team/planted.txt" 2>/dev/null; then
    echo 'a member wrote into a circle directory only the service writes' >&2
    return 1
  fi
  if runuser -u bc_person_outsider -- cat "$circle_memory" >/dev/null 2>&1; then
    echo 'non-member read the circle memory' >&2
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
echo 'workspace-ownership: preserved across repeated workspace syncs; circle memory, read-only circles and private boundaries verified'
