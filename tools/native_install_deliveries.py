"""Send a list of requests as the member and keep what the agent delivers for each.

Each request is answered in turn: its message is sent over the member's own
socket, with any attachments carried the way the web app carries them, the
agent's task for it is followed to an end, and every file the agent posted in
reply is copied out of the guest under the request's name, beside the request,
the replies and the task's own ledger. Files the agent wrote in the workspace
during the request and did not post are kept beside them, under workspace/.
"""

import hashlib
import json
import mimetypes
import shlex
import shutil
import time
import urllib.parse
from pathlib import Path

from native_install_rig import COMPANY_CONDITION_PATH, MESSENGER_DATABASE_PATH, SHARE_PATH

ENDED_STATUSES = {"completed", "failed", "cancelled", "waiting_user_input", "blocked", "interrupted", "waiting_approval"}
STILL_OPEN_STATUSES = {"waiting_user_input", "blocked", "waiting_approval", "interrupted"}
POLL_SECONDS = 15
SETTLE_SECONDS = 45
RELAY_ADDRESS = "http://127.0.0.1:3000"
WORKSPACE_ROOT = "/workspace"
WORKSPACE_DIRECTORY_NAME = "workspace"

TASK_QUERY = """
select coalesce(json_agg(row_to_json(found)), '[]'::json) from (
  select task_run_id, status, created_at, updated_at, failure_reason, left(coalesce(result, ''), 4000) as result
  from task_run
  where position(:'snippet' in prompt) > 0 and created_at >= to_timestamp(:sent_at)
  order by created_at
) found
"""

EVENTS_QUERY = """
select coalesce(json_agg(row_to_json(found)), '[]'::json) from (
  select name, created_at, left(body, 1500) as body from task_event
  where task_run_id = :'task' order by created_at
) found
"""

REPLIES_QUERY = """
select coalesce(json_agg(row_to_json(found)), '[]'::json) from (
  select encode(answer.id, 'hex') as id, answer.created_at, answer.content, answer.tags
  from events answer join events asked on encode(asked.id, 'hex') = :'message'
  where answer.kind = 9 and answer.pubkey <> asked.pubkey and answer.created_at >= asked.created_at
  order by answer.created_at
) found
"""

QUERY_COMMAND = r"""
url="$(sed -n 's/^DATABASE_URL=//p' %(environment)s 2>/dev/null | head -1)"
psql "$url" -tAX %(variables)s -f %(query)s 2>&1
"""

FETCH_COMMAND = r"""
set -u
target=%(target)s
mkdir -p "$(dirname "$target")"
curl -sS --max-time 120 -o "$target" -H "Host: %(host)s" "%(relay)s%(path)s" || true
if [ "$(sha256sum "$target" 2>/dev/null | cut -d' ' -f1)" != "%(digest)s" ]; then
  rm -f "$target"
  for candidate in $(find /var/lib/internkim /workspace /home /srv/internkim -xdev -type f -size %(size)sc 2>/dev/null); do
    if [ "$(sha256sum "$candidate" | cut -d' ' -f1)" = "%(digest)s" ]; then cp "$candidate" "$target"; break; fi
  done
fi
for candidate in $(find /workspace /home -xdev -type f -size %(size)sc 2>/dev/null); do
  if [ "$(sha256sum "$candidate" | cut -d' ' -f1)" = "%(digest)s" ]; then echo "workspace=$candidate"; fi
done
test -s "$target" && echo fetched=yes || echo fetched=no
"""


WORKSPACE_SNAPSHOT_COMMAND = r"""
set -u
target=%(target)s
skipped=" %(carried)s "
mkdir -p "$target"
find %(root)s -xdev -type f -newermt @%(since)s \
  -not -path '%(root)s/.blueclaw/*' -not -path '*/.venv/*' -not -path '*/node_modules/*' -not -path '*/.git/*' 2>/dev/null |
while IFS= read -r file; do
  digest="$(sha256sum "$file" | cut -d' ' -f1)"
  case "$skipped" in *" $digest "*) continue ;; esac
  cp --parents "$file" "$target" && echo "kept=${file#/}"
done
"""


def load_requests(path):
    document = json.loads(Path(path).read_text())
    if not isinstance(document, list):
        raise ValueError(f"{path} must hold a JSON list of requests")
    base = Path(path).resolve().parent
    return [parse_request(base, entry) for entry in document]


def parse_request(base, entry):
    name = entry.get("name") if isinstance(entry, dict) else None
    text = entry.get("text") if isinstance(entry, dict) else None
    if not isinstance(name, str) or Path(name).name != name or name in ("", ".", ".."):
        raise ValueError(f"a request needs a name that is a single path segment, got {name!r}")
    if not isinstance(text, str) or not text.strip():
        raise ValueError(f"request {name} needs a text")
    return {
        **entry,
        "attachments": [attachment_file(base, attachment) for attachment in entry.get("attachments", [])],
    }


def first_line_snippet(request):
    return request.get("snippet") or request["text"].strip().splitlines()[0][:40]


def workspace_snapshot_command(target, since_epoch, carried_digests):
    return WORKSPACE_SNAPSHOT_COMMAND % {
        "target": shlex.quote(target),
        "carried": " ".join(sorted(digest for digest in carried_digests if digest)),
        "root": WORKSPACE_ROOT,
        "since": int(since_epoch),
    }


def kept_paths(shell_output):
    return [line.split("=", 1)[1] for line in shell_output.splitlines() if line.startswith("kept=")]


def attachment_file(base, attachment):
    described = attachment if isinstance(attachment, dict) else {"path": attachment}
    path = described["path"]
    return {
        "path": str((base / path).resolve()),
        "filename": Path(path).name,
        "contentType": described.get("contentType") or mimetypes.guess_type(path)[0] or "application/octet-stream",
    }


def media_of(tags):
    found = []
    for tag in tags or []:
        if not tag or tag[0] != "imeta":
            continue
        fields = {}
        for entry in tag[1:]:
            name, _, value = entry.partition(" ")
            fields[name] = value
        if fields.get("url"):
            found.append(fields)
    return found


class Deliveries:
    def __init__(self, rig, plane, deliveries_directory):
        self.rig = rig
        self.plane = plane
        self.directory = Path(deliveries_directory).resolve()
        self.directory.mkdir(parents=True, exist_ok=True)
        self.member_id = plane.member_identity(rig.company_id)

    @property
    def machine(self):
        return self.rig.machine

    def query(self, environment_path, sql, **variables):
        query_name = f"query-{hashlib.sha256((sql + json.dumps(variables, sort_keys=True)).encode()).hexdigest()[:16]}.sql"
        (self.machine.share_directory / query_name).write_text(sql)
        settings = " ".join(f"-v {name}={shlex.quote(str(value))}" for name, value in variables.items())
        completed = self.machine.shell(
            QUERY_COMMAND % {"environment": environment_path, "variables": settings, "query": f"{SHARE_PATH}/{query_name}"}
        )
        text = completed.stdout.strip()
        try:
            return json.loads(text.splitlines()[-1]) if text else []
        except (json.JSONDecodeError, IndexError):
            return [{"unreadable": text[-1000:]}]

    def deliver_all(self, requests):
        summary = []
        for request in requests:
            summary.append(self.deliver(request))
            (self.directory / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2))
        return summary

    def deliver(self, request):
        case_directory = self.directory / request["name"]
        case_directory.mkdir(parents=True, exist_ok=True)
        (case_directory / "request.txt").write_text(request["text"])
        sent_at = time.time() - 5
        print(f"  → {request['name']}", flush=True)
        session = self.plane.member_session(
            self.rig.company_id, request["text"], attachments=request["attachments"], member_id=self.member_id
        )
        sent = session.get("sent") or {}
        message_id = sent.get("messageID") or ""
        record = {
            "name": request["name"],
            "messageID": message_id,
            "sentAttachments": session.get("attachments"),
            "refused": session.get("refused"),
            "sentStatus": sent.get("status"),
            "sentBody": sent.get("body"),
        }
        if not message_id:
            record["outcome"] = "not sent"
            print(f"    not sent: {sent.get('status')} {json.dumps(sent.get('body'), ensure_ascii=False)[:300]}", flush=True)
            return self.keep(case_directory, record)
        snippet = first_line_snippet(request)
        tasks = self.follow_the_task(snippet, sent_at, message_id, request.get("timeoutSeconds", 1800))
        time.sleep(SETTLE_SECONDS)
        tasks = self.query(COMPANY_CONDITION_PATH, TASK_QUERY, snippet=snippet, sent_at=int(sent_at)) or tasks
        replies = self.query(MESSENGER_DATABASE_PATH, REPLIES_QUERY, message=message_id)
        record["tasks"] = tasks
        record["replies"] = replies
        record["events"] = {
            task["task_run_id"]: self.query(COMPANY_CONDITION_PATH, EVENTS_QUERY, task=task["task_run_id"])
            for task in tasks
            if isinstance(task, dict) and task.get("task_run_id")
        }
        record["files"] = self.fetch_media(case_directory, replies)
        record["workspaceFiles"] = self.snapshot_workspace(case_directory, sent_at, record["files"])
        record["elapsedSeconds"] = round(time.time() - sent_at)
        record["outcome"] = tasks[-1].get("status") if tasks and isinstance(tasks[-1], dict) else "no task"
        if record["outcome"] in STILL_OPEN_STATUSES or record["outcome"] in ("running", "planned", "no task"):
            self.plane.member_session(self.rig.company_id, "/stop-all")
            time.sleep(10)
        print(f"    {record['outcome']} in {record['elapsedSeconds']}s, files {[file.get('name') for file in record['files']]}", flush=True)
        return self.keep(case_directory, record)

    def snapshot_workspace(self, case_directory, sent_at, delivered_files):
        staged = f"deliveries/{case_directory.name}/{WORKSPACE_DIRECTORY_NAME}"
        carried = [file.get("x", "") for file in delivered_files]
        completed = self.machine.shell(workspace_snapshot_command(f"{SHARE_PATH}/{staged}", sent_at, carried))
        source = self.machine.share_directory / staged
        if source.is_dir():
            shutil.copytree(source, case_directory / WORKSPACE_DIRECTORY_NAME, dirs_exist_ok=True)
            shutil.rmtree(source)
        return kept_paths(completed.stdout)

    def follow_the_task(self, snippet, sent_at, message_id, timeout_seconds):
        started = time.monotonic()
        tasks = []
        while time.monotonic() < started + timeout_seconds:
            tasks = self.query(COMPANY_CONDITION_PATH, TASK_QUERY, snippet=snippet, sent_at=int(sent_at))
            statuses = [task.get("status") for task in tasks if isinstance(task, dict)]
            if statuses and all(status in ENDED_STATUSES for status in statuses):
                return tasks
            if not statuses and time.monotonic() > started + 300:
                if self.query(MESSENGER_DATABASE_PATH, REPLIES_QUERY, message=message_id):
                    print("    no task carries the request's first line; the agent replied, so the wait ends", flush=True)
                    return tasks
            time.sleep(POLL_SECONDS)
        return tasks

    def fetch_media(self, case_directory, replies):
        files = []
        for reply in replies if isinstance(replies, list) else []:
            for media in media_of(reply.get("tags") if isinstance(reply, dict) else []):
                files.append(self.fetch_one(case_directory, media))
        return files

    def fetch_one(self, case_directory, media):
        address = urllib.parse.urlsplit(media["url"])
        digest = media.get("x", "")
        staged = f"deliveries/{case_directory.name}/{digest}"
        completed = self.machine.shell(
            FETCH_COMMAND
            % {
                "target": f"{SHARE_PATH}/{staged}",
                "host": address.netloc,
                "relay": RELAY_ADDRESS,
                "path": address.path,
                "digest": digest,
                "size": media.get("size", "0"),
            }
        )
        workspace_paths = [line.split("=", 1)[1] for line in completed.stdout.splitlines() if line.startswith("workspace=")]
        name = media.get("name") or media.get("filename") or media.get("alt") or ""
        if not name and workspace_paths:
            name = Path(workspace_paths[0]).name
        if not name:
            name = digest + (mimetypes.guess_extension(media.get("m", "")) or "")
        source = self.machine.share_directory / staged
        if source.is_file():
            shutil.copyfile(source, case_directory / name)
        return {**media, "name": name, "workspacePaths": workspace_paths, "fetched": source.is_file()}

    def keep(self, case_directory, record):
        (case_directory / "result.json").write_text(json.dumps(record, ensure_ascii=False, indent=2, default=str))
        return record
