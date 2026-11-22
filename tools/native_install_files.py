"""What the native-install rig compares of a company's files across a take-over.

A manifest is every entry under the roots a backup carries, read as root in the guest: its
type, size, mode, owner and group by name and by number, link target, and the SHA-256 of a
regular file. The rig takes one on the serving box right after the snapshot it takes over
from, one just before that box goes dark, and one on the box that took over, then compares
them here.
"""

from dataclasses import dataclass

BACKUP_ROOTS = (
    ("state", "/var/lib/internkim"),
    ("roster", "/var/lib/internkim-roster"),
    ("relay", "/var/lib/internkim-relay"),
    ("configuration", "/etc/internkim"),
    ("administration", "/root/.internkim"),
    ("workspace", "/workspace"),
)

COMPARED_FIELDS = ("type", "size", "mode", "owner", "group", "target", "digest")

SEEDED_COMPANY = "000000cc-0000-0000-0000-000000000001"
SEEDED_FIRST_MEMBER = "000000ee-0000-0000-0000-000000000001"
SQLITE_COPY = (
    "a SQLite database the backup copies as one consolidated file through SQLite's own snapshot "
    "(hostbackup.SQLiteSnapshots), so its bytes and size are not the live file's"
)
SQLITE_SIDE_FILE = "a SQLite -wal or -shm file, which the backup folds into the database it copies and SQLite makes again when it opens it"

DIFFERING_BY_DESIGN = {
    ("state", "box"): "each box keeps its own identity, pairing state and session under box/, which no backup carries to another box",
    ("state", "media/.vgwlocks"): "the media store's lock files, which the backup excludes (hostFileRoots in internal/companyhost/backup.go)",
    ("state", ".file-journal-barrier"): "the file follower's own working directory, made when it starts following (internal/filestream/barrier.go)",
    ("workspace", ".blueclaw/state/record-index.db-wal"): SQLITE_SIDE_FILE,
    ("workspace", ".blueclaw/state/record-index.db-shm"): SQLITE_SIDE_FILE,
    ("workspace", f"private/protected/{SEEDED_FIRST_MEMBER}/memory.db-wal"): SQLITE_SIDE_FILE,
    ("workspace", f"private/protected/{SEEDED_FIRST_MEMBER}/memory.db-shm"): SQLITE_SIDE_FILE,
}

CONTENT_DIFFERS_BY_DESIGN = {
    ("configuration", "agent-key"): "the session key of the box that serves, which the take-over writes for this box",
    ("state", f"companies/{SEEDED_COMPANY}/secrets/agent-key"): "the same session key, kept with the company",
    ("state", f"companies/{SEEDED_COMPANY}/connection.json"): "the connection file, which carries the serving box's session",
    ("administration", "state/internkim.sqlite"): SQLITE_COPY,
    ("workspace", ".blueclaw/state/record-index.db"): SQLITE_COPY,
    ("workspace", f"private/protected/{SEEDED_FIRST_MEMBER}/memory.db"): SQLITE_COPY,
    ("workspace", ".blueclaw/skill-index.json"): (
        "the agent's cache of skill embeddings, keyed by each SKILL.md's path and SHA-256 and rewritten from the skills "
        "on hand at start-up and at every skill search (refresh in bluecollar loop/skill_retriever.go), so its size "
        "follows the last search"
    ),
}

ARRIVING_BY_DESIGN = {
    ("state", "media/buzz-media/probe"): "the media store writes a probe object each time it starts",
    ("state", "media/buzz-media/packs"): "the media store packs its start-up probe objects; a pack the backup carried still has to arrive unchanged",
}

CONTENT_FIELDS = ("size", "digest")

MANIFEST_COMMAND = r"""
set -u
for root in %(roots)s; do
  role=${root%%%%:*}; path=${root#*:}
  test -d "$path" || continue
  find "$path" -mindepth 1 -printf "entry\t$role\t%%P\t%%y\t%%s\t%%m\t%%u\t%%g\t%%U\t%%G\t%%l\n"
  find "$path" -mindepth 1 -type f -print0 | xargs -0 -r sha256sum | sed "s|^|digest\t$role\t$path\t|"
done > %(output)s
"""

SPREAD_COMMAND = r"""
set -eu
people_root=/workspace/private/people
people=$(ls -1 "$people_root" | head -3)
test "$(printf '%s\n' "$people" | wc -l)" -ge 3
for person in $people; do
  home="$people_root/$person"
  owner=$(stat -c %U "$home")
  runuser -u "$owner" -- sh -c "printf 'private note of %s\n' '$person' > '$home/rig-private-note.txt'"
done
first=$(printf '%s\n' "$people" | head -1)
second=$(printf '%s\n' "$people" | sed -n 2p)
first_home="$people_root/$first"
first_owner=$(stat -c %U "$first_home")
runuser -u "$first_owner" -- sh -c "
  cd '$first_home'
  printf '회의록\n' > '회의록 메모.md'
  printf 'a name with spaces\n' > 'a file with spaces.txt'
  printf 'read by its owner and group\n' > rig-mode-0640.txt
  chmod 0640 rig-mode-0640.txt
  ln -s '회의록 메모.md' rig-link-to-the-minutes
  mkdir rig-empty-directory
"
protected="/workspace/private/protected/$first"
printf '{"source": "rig"}\n' > "$protected/rig-source-of-truth.json"
chown --reference="$protected" "$protected/rig-source-of-truth.json"
chmod 0640 "$protected/rig-source-of-truth.json"
circle=""; outsider=""
for circle_directory in /workspace/circles/*/; do
  candidate=$(basename "$circle_directory")
  for person in $(ls -1 "$people_root"); do
    owner=$(stat -c %U "$people_root/$person")
    if ! id -nG "$owner" | tr ' ' '\n' | grep -qx "bc_circle_$candidate"; then
      circle=$candidate; outsider=$owner; break 2
    fi
  done
done
test -n "$circle"
printf 'what the circle remembers\n' > "/workspace/circles/$circle/rig-circle-memory.md"
chown --reference="/workspace/circles/$circle" "/workspace/circles/$circle/rig-circle-memory.md"
chmod 0640 "/workspace/circles/$circle/rig-circle-memory.md"
printf 'shared with everyone\n' > /workspace/shared/rig-shared-note.txt
chown --reference=/workspace/shared /workspace/shared/rig-shared-note.txt
chmod 0664 /workspace/shared/rig-shared-note.txt
printf 'first=%s\nfirst-owner=%s\nsecond=%s\nsecond-owner=%s\ncircle=%s\noutsider=%s\n' \
  "$first" "$first_owner" "$second" "$(stat -c %U "$people_root/$second")" "$circle" "$outsider"
"""

ACCESS_WALLS_COMMAND = r"""
people_root=/workspace/private/people
attempt() { if runuser -u "$2" -- sh -c "$3" >/dev/null 2>&1; then echo "$1=allowed"; else echo "$1=refused"; fi; }
attempt own-private-file %(first_owner)s "cat $people_root/%(first)s/rig-private-note.txt"
attempt another-persons-private-file %(first_owner)s "cat $people_root/%(second)s/rig-private-note.txt"
attempt circle-directory-by-an-outsider %(outsider)s "ls /workspace/circles/%(circle)s"
attempt circle-memory-by-an-outsider %(outsider)s "cat /workspace/circles/%(circle)s/rig-circle-memory.md"
attempt write-under-protected %(first_owner)s "touch /workspace/private/protected/%(first)s/rig-write-attempt"
test -e "/workspace/private/protected/%(first)s/rig-write-attempt" && echo protected-file-written=yes || echo protected-file-written=no
"""

EXPECTED_ACCESS = {
    "own-private-file": "allowed",
    "another-persons-private-file": "refused",
    "circle-directory-by-an-outsider": "refused",
    "circle-memory-by-an-outsider": "refused",
    "write-under-protected": "refused",
    "protected-file-written": "no",
}

SPREAD_PATHS = (
    "rig-private-note.txt",
    "회의록 메모.md",
    "a file with spaces.txt",
    "rig-mode-0640.txt",
    "rig-link-to-the-minutes",
    "rig-empty-directory",
    "rig-source-of-truth.json",
    "rig-circle-memory.md",
    "rig-shared-note.txt",
)


def manifest_command(output_path):
    roots = " ".join(f"{role}:{path}" for role, path in BACKUP_ROOTS)
    return MANIFEST_COMMAND % {"roots": roots, "output": output_path}


def manifest_of(reading):
    """Each entry of a manifest reading, keyed by (root, relative path)."""
    entries = {}
    digests = {}
    for line in reading.splitlines():
        kind, _, rest = line.partition("\t")
        if kind == "entry":
            role, path, kind_of_entry, size, mode, owner, group, uid, gid, target = rest.split("\t")
            entries[(role, path)] = {
                "type": kind_of_entry,
                "size": size if kind_of_entry in ("f", "l") else "",
                "mode": mode,
                "owner": owner,
                "group": group,
                "uid": uid,
                "gid": gid,
                "target": target,
                "digest": "",
            }
        elif kind == "digest":
            role, root_path, summed = rest.split("\t", 2)
            digest, _, absolute = summed.partition("  ")
            digests[(role, absolute.removeprefix(root_path + "/"))] = digest.lstrip("\\")
    for key, digest in digests.items():
        if key in entries:
            entries[key]["digest"] = digest
    return entries


def under(key, named):
    role, path = key
    for (named_role, named_path), reason in named.items():
        if role == named_role and (path == named_path or path.startswith(named_path + "/")):
            return reason
    return ""


def compared(entry, fields):
    return None if entry is None else tuple(entry[field] for field in fields)


@dataclass
class Comparison:
    compared: int
    differences: list
    changed_after_the_backup: int
    exempted: int


def compare(at_backup, before_dark, after):
    """What differs on the box that took over from the serving box at the backup.

    A path that changed on the serving box between the backup and going dark may be found as
    it was at either moment; anything else must match the backup exactly, apart from the
    paths the lists above name with their reasons.
    """
    differences = []
    changed_after_the_backup = 0
    exempted = 0
    keys = sorted(set(at_backup) | set(before_dark) | set(after))
    for key in keys:
        is_new = key not in at_backup and key not in before_dark
        if under(key, DIFFERING_BY_DESIGN) or (is_new and under(key, ARRIVING_BY_DESIGN)):
            exempted += 1
            continue
        fields = COMPARED_FIELDS
        if key in CONTENT_DIFFERS_BY_DESIGN:
            fields = tuple(field for field in COMPARED_FIELDS if field not in CONTENT_FIELDS)
        expected = {compared(at_backup.get(key), fields)}
        if compared(before_dark.get(key), COMPARED_FIELDS) != compared(at_backup.get(key), COMPARED_FIELDS):
            changed_after_the_backup += 1
            expected.add(compared(before_dark.get(key), fields))
        if compared(after.get(key), fields) not in expected:
            differences.append(describe_difference(key, at_backup.get(key), after.get(key)))
    return Comparison(len(keys) - exempted, differences, changed_after_the_backup, exempted)


def exemption_list():
    named = [("not compared", DIFFERING_BY_DESIGN), ("content not compared", CONTENT_DIFFERS_BY_DESIGN), ("may newly appear", ARRIVING_BY_DESIGN)]
    return [f"{kind} {role}/{path}: {reason}" for kind, listing in named for (role, path), reason in listing.items()]


def describe_difference(key, expected, found):
    role, path = key
    if expected is None:
        return f"extra {role}/{path}: {row_of(found)}"
    if found is None:
        return f"missing {role}/{path}: {row_of(expected)}"
    fields = [f"{field} {expected[field]!r} -> {found[field]!r}" for field in COMPARED_FIELDS if expected[field] != found[field]]
    return f"differs {role}/{path}: {', '.join(fields)}"


def row_of(entry):
    return " ".join(f"{field}={entry[field]}" for field in COMPARED_FIELDS + ("uid", "gid") if entry[field])


def numbers_by_name(manifest):
    """The numeric ids each owner and group name carries in a manifest."""
    numbers = {}
    for entry in manifest.values():
        numbers.setdefault(("user", entry["owner"]), set()).add(entry["uid"])
        numbers.setdefault(("group", entry["group"]), set()).add(entry["gid"])
    return numbers


def renumbered_names(before, after):
    """Each name whose number differs between two manifests, and whether every name keeps one number on each box."""
    numbers_before, numbers_after = numbers_by_name(before), numbers_by_name(after)
    renumbered = []
    is_consistent = True
    for name in sorted(set(numbers_before) & set(numbers_after)):
        if len(numbers_before[name]) != 1 or len(numbers_after[name]) != 1:
            is_consistent = False
        if numbers_before[name] != numbers_after[name]:
            renumbered.append(f"{name[0]} {name[1]}: {','.join(sorted(numbers_before[name]))} -> {','.join(sorted(numbers_after[name]))}")
    return renumbered, is_consistent


def spread_names(reading):
    return dict(line.split("=", 1) for line in reading.splitlines() if "=" in line)


WALL_NAMES = ("first", "first-owner", "second", "second-owner", "circle", "outsider")


def walls_can_be_tried(spread):
    return all(spread.get(name) for name in WALL_NAMES)


def walls_hold(reading):
    observed = spread_names(reading)
    return all(observed.get(name) == wanted for name, wanted in EXPECTED_ACCESS.items())


def spread_found_in(manifest):
    """The spread's own paths, each with whether the manifest holds one by that name."""
    held_names = {path.rsplit("/", 1)[-1] for _, path in manifest} | {
        "/".join(path.rsplit("/", 2)[-2:]) for _, path in manifest
    }
    return {path: path in held_names for path in SPREAD_PATHS}
