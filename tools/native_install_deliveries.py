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
import os
import shlex
import shutil
import subprocess
import time
import urllib.parse
from pathlib import Path

from native_install_rig import COMPANY_CONDITION_PATH, MESSENGER_DATABASE_PATH, REPOSITORY_ROOT, SHARE_PATH

ENDED_STATUSES = {"completed", "failed", "cancelled", "waiting_user_input", "blocked", "interrupted", "waiting_approval"}
STILL_OPEN_STATUSES = {"waiting_user_input", "blocked", "waiting_approval", "interrupted"}
POLL_SECONDS = 15
SETTLE_SECONDS = 45
RELAY_ADDRESS = "http://127.0.0.1:3000"
WORKSPACE_ROOT = "/workspace"
WORKSPACE_DIRECTORY_NAME = "workspace"
JUDGE_SCRIPT = "tests/e2e/judge-delivery.ts"
MESSENGER = "buzz"
TASK_FIELDS = ("name", "requesterEmail", "instruction", "answers", "assertions", "cleanup")

TASK_QUERY = """
select coalesce(json_agg(row_to_json(found)), '[]'::json) from (
  select task_run_id, status, created_at, updated_at, failure_reason, left(coalesce(result, ''), 4000) as result
  from task_run
  where origin_conversation_id = :'messenger' || ':' || :'conversation' and created_at >= (:'posted_at')::timestamptz
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


def load_requests(path, only=None):
    source = Path(path).resolve()
    if source.is_dir():
        base = source
        document = [json.loads(task.read_text()) for task in sorted(source.glob("*.json"))]
    else:
        base = source.parent
        document = json.loads(source.read_text())
    if not isinstance(document, list):
        raise ValueError(f"{path} must hold a JSON list of requests or be a directory of tasks")
    requests = [parse_request(base, entry) for entry in document]
    if not only:
        return requests
    chosen = set(only.split(","))
    unknown = chosen - {request["name"] for request in requests}
    if unknown:
        raise ValueError(f"no request named {', '.join(sorted(unknown))} in {path}")
    return [request for request in requests if request["name"] in chosen]


def parse_request(base, entry):
    name = entry.get("name") if isinstance(entry, dict) else None
    instruction = entry.get("instruction") if isinstance(entry, dict) else None
    if not isinstance(name, str) or Path(name).name != name or name in ("", ".", ".."):
        raise ValueError(f"a request needs a name that is a single path segment, got {name!r}")
    if not isinstance(instruction, str) or not instruction.strip():
        raise ValueError(f"request {name} needs an instruction")
    return {
        **entry,
        "attachments": [attachment_file(base, attachment) for attachment in entry.get("attachments", [])],
    }


def status_of_latest(tasks):
    return tasks[-1].get("status") if tasks and isinstance(tasks[-1], dict) else "no task"


def ledger_state(tasks):
    return [(task.get("task_run_id"), task.get("status"), task.get("updated_at")) for task in tasks if isinstance(task, dict)]


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
        rig.give_the_guest_the_messenger_probe()

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

    def requester_ready_to_send(self, email):
        member_id = self.plane.member_identity(self.rig.company_id, email)
        self.plane.wait_until_the_member_holds_a_messenger_credential(member_id)
        self.rig.wait_until_the_relay_knows(email)
        return member_id

    def deliver(self, request):
        case_directory = self.directory / request["name"]
        case_directory.mkdir(parents=True, exist_ok=True)
        (case_directory / "request.txt").write_text(request["instruction"])
        email = request.get("requesterEmail") or self.plane.administrator_email
        member_id = self.requester_ready_to_send(email)
        sent_at = time.time() - 5
        print(f"  → {request['name']} as {email}", flush=True)
        session = self.plane.member_session(
            self.rig.company_id,
            request["instruction"],
            attachments=request["attachments"],
            member_id=member_id,
            time_zone=request.get("timeZone", ""),
            email=email,
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
        conversation = (sent.get("body") or {}).get("conversationID") or ""
        posted_at = (sent.get("body") or {}).get("postedAt") or ""
        timeout_seconds = request.get("timeoutSeconds", 1800)
        tasks = self.follow_the_task(conversation, posted_at, message_id, timeout_seconds)
        for answer in request.get("answers", []):
            if status_of_latest(tasks) not in STILL_OPEN_STATUSES:
                break
            print(f"    {status_of_latest(tasks)}, answering {answer!r}", flush=True)
            self.plane.member_session(
                self.rig.company_id, answer, member_id=member_id, time_zone=request.get("timeZone", ""), email=email,
                parent_id=message_id,
            )
            tasks = self.follow_the_task(conversation, posted_at, message_id, timeout_seconds, before=ledger_state(tasks))
        time.sleep(SETTLE_SECONDS)
        tasks = self.query(COMPANY_CONDITION_PATH, TASK_QUERY, messenger=MESSENGER, conversation=conversation, posted_at=posted_at) or tasks
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
        record["outcome"] = status_of_latest(tasks)
        if record["outcome"] in STILL_OPEN_STATUSES or record["outcome"] in ("running", "planned", "no task"):
            self.plane.member_session(self.rig.company_id, "/stop-all", member_id=member_id, email=email)
            time.sleep(10)
        print(f"    {record['outcome']} in {record['elapsedSeconds']}s, files {[file.get('name') for file in record['files']]}", flush=True)
        if "assertions" in request:
            record["inboxes"] = self.read_inboxes(request["assertions"], posted_at)
            self.keep(case_directory, record)
            record["judgement"] = self.judge(case_directory, request)
            print(f"    {'passed' if record['judgement'].get('passed') else 'failed'}", flush=True)
        return self.keep(case_directory, record)

    def read_inboxes(self, assertions, since):
        emails = sorted({assertion["inboxOf"]["email"] for assertion in assertions if "inboxOf" in assertion})
        return {email: self.rig.ask_the_messenger("inbox_since", email=email, since=since).get("bodies", []) for email in emails}

    def judge(self, case_directory, request):
        task = {field: request[field] for field in TASK_FIELDS if field in request}
        (case_directory / "task.json").write_text(json.dumps(task, ensure_ascii=False, indent=2))
        completed = subprocess.run(
            ["bun", "run", JUDGE_SCRIPT, str(case_directory)],
            cwd=REPOSITORY_ROOT / "web", env=os.environ | self.plane.test_environment(),
            capture_output=True, text=True, timeout=300,
        )
        for line in reversed(completed.stdout.splitlines()):
            if line.startswith("{"):
                return json.loads(line)
        return {"passed": False, "refused": (completed.stderr.strip() or completed.stdout.strip())[-900:]}

    def snapshot_workspace(self, case_directory, sent_at, delivered_files):
        staged = f"deliveries/{case_directory.name}/{WORKSPACE_DIRECTORY_NAME}"
        carried = [file.get("x", "") for file in delivered_files]
        completed = self.machine.shell(workspace_snapshot_command(f"{SHARE_PATH}/{staged}", sent_at, carried))
        source = self.machine.share_directory / staged
        if source.is_dir():
            shutil.copytree(source, case_directory / WORKSPACE_DIRECTORY_NAME, dirs_exist_ok=True)
            shutil.rmtree(source)
        return kept_paths(completed.stdout)

    def follow_the_task(self, conversation, posted_at, message_id, timeout_seconds, before=None):
        started = time.monotonic()
        tasks = []
        while time.monotonic() < started + timeout_seconds:
            tasks = self.query(COMPANY_CONDITION_PATH, TASK_QUERY, messenger=MESSENGER, conversation=conversation, posted_at=posted_at)
            statuses = [task.get("status") for task in tasks if isinstance(task, dict)]
            has_moved = before is None or ledger_state(tasks) != before
            if statuses and all(status in ENDED_STATUSES for status in statuses) and has_moved:
                return tasks
            if not statuses and time.monotonic() > started + 300:
                if self.query(MESSENGER_DATABASE_PATH, REPLIES_QUERY, message=message_id):
                    print("    no task runs in the conversation; the agent replied, so the wait ends", flush=True)
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
