import base64
import json
import os
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlencode


workspace = Path("/mnt/shared/workspace")
chatd_origin = "http://172.31.0.1:18090"
requester_email = sys.argv[1]
instruction = base64.b64decode(sys.argv[2]).decode("utf-8")
settle_timeout_seconds = 720


def request(path, body=None, accepted=(200,), timeout=15, origin="http://127.0.0.1:8080"):
    command = ["curl", "--silent", "--show-error", "--max-time", str(timeout), "--write-out", "\n%{http_code}"]
    if body is not None:
        command += ["-H", "Content-Type: application/json", "-d", "@-"]
    response = subprocess.run(command + [origin + path], input=json.dumps(body) if body is not None else None, capture_output=True, text=True, check=True)
    payload, status = response.stdout.rsplit("\n", 1)
    if int(status) not in accepted:
        raise RuntimeError(f"{path}: HTTP {status}: {payload}")
    return json.loads(payload) if int(status) == 200 else {"status": int(status), "body": payload}


def wait_for(read, description, timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = read()
        if value:
            return value
        time.sleep(3)
    raise RuntimeError(f"timed out waiting for {description}")


def requester_secret():
    with tempfile.TemporaryDirectory(prefix="pilot-member-") as directory:
        Path(directory, "key-seed").symlink_to("/root/.internkim/secrets/buzz-key-seed")
        minted = subprocess.run([str(workspace / "host/buzz/my-buzz-key"), requester_email], env={**os.environ, "BUZZ_STATE_DIRECTORY": directory}, capture_output=True, text=True)
        if minted.returncode != 0:
            raise RuntimeError(f"my-buzz-key {requester_email}: exit {minted.returncode}: {minted.stderr.strip()}")
        written = list(Path(directory).glob("buzz-key-*.txt"))
        assert len(written) == 1, "my-buzz-key must write exactly one key"
        return written[0].read_text().strip()


def send_direct_message(secret, text):
    sent_at = datetime.now(timezone.utc)
    return sent_at, request("/v1/platform/buzz/dm.send", {"userSecretHex": secret, "message": text}, origin=chatd_origin)


approved_calls = []


def approve_as_the_requester(task_run_id):
    approved_calls.append(task_run_id)
    request("/admin/api/run/approve", {"taskRunID": task_run_id, "decision": "confirm"}, accepted=(200, 400), timeout=600)


def settled_run_after(person_id, conversation_id, sent_at):
    for listed in request("/admin/api/run?viewerIsAdmin=true"):
        if listed["requesterPersonID"] != person_id or listed["status"] in {"planned", "running"}:
            continue
        if listed.get("originConversationID") != conversation_id or datetime.fromisoformat(listed["createdAt"]) < sent_at:
            continue
        if listed["status"] == "waiting_approval":
            approve_as_the_requester(listed["taskRunID"])
            return None
        return request("/admin/api/run/detail?" + urlencode({"taskRunID": listed["taskRunID"], "viewerIsAdmin": "true"}))
    return None


def events_named(detail, name):
    return [event for event in detail["taskEvents"] if event["name"] == name]


def requested_tools(detail):
    return [event["name"][len("tool.") : -len(".requested")] for event in detail["taskEvents"] if event["name"].startswith("tool.") and event["name"].endswith(".requested")]


def main():
    policy = request("/admin/api/policy")
    people = [person for person in policy["people"] if requester_email in person["emails"]]
    assert len(people) == 1, f"{requester_email} must resolve to exactly one person"
    person_id = people[0]["personID"]
    secret = requester_secret()
    conversation_id = "buzz:" + send_direct_message(secret, "/stop")[1]["channelID"]
    sent_at, _ = send_direct_message(secret, instruction)
    detail = wait_for(lambda: settled_run_after(person_id, conversation_id, sent_at), "the request to settle", settle_timeout_seconds)
    task_run = detail["taskRun"]
    print(resultLine := "PILOT-RESULT " + json.dumps({
        "status": task_run["status"],
        "taskRunID": task_run["taskRunID"],
        "requestedTools": requested_tools(detail),
        "llmCalls": [json.loads(event["body"]) for event in events_named(detail, "llm.call")],
        "result": task_run.get("result") or "",
        "failureReason": task_run.get("failureReason") or "",
        "turns": len(events_named(detail, "agent.action")),
        "approvals": len(approved_calls),
        "detail": detail,
    }, ensure_ascii=False), flush=True)


main()
