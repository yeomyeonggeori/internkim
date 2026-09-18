import json
import os
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlencode


evidence = Path(sys.argv[1])
evidence.mkdir(parents=True, exist_ok=True)
workspace = Path("/mnt/shared/workspace")
chatd_origin = "http://172.31.0.1:18090"
requester_email = "member4@example.com"
create_request = "매일 오전 9시에 그날의 할 일 목록을 이 대화로 보내줘."
cancel_request = "방금 만든 매일 아침 예약은 취소해줘."


def request(path, body=None, accepted=(200,), timeout=15, origin="http://127.0.0.1:8080"):
    command = ["curl", "--silent", "--show-error", "--max-time", str(timeout), "--write-out", "\n%{http_code}"]
    if body is not None:
        command += ["-H", "Content-Type: application/json", "-d", "@-"]
    response = subprocess.run(command + [origin + path], input=json.dumps(body) if body is not None else None, capture_output=True, text=True, check=True)
    payload, status = response.stdout.rsplit("\n", 1)
    if int(status) not in accepted:
        raise RuntimeError(f"{path}: HTTP {status}: {payload}")
    return json.loads(payload) if int(status) == 200 else {"status": int(status), "body": payload}


def save(name, value):
    (evidence / (name + ".json")).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    return value


def wait_for(read, description, timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = read()
        if value:
            return value
        time.sleep(3)
    raise RuntimeError(f"timed out waiting for {description}")


def schedules_of(person_id):
    query = urlencode({"creatorPersonID": person_id, "includeExpired": "true", "pageSize": 200})
    return {item["taskScheduleID"]: item for item in request("/admin/api/schedule?" + query)["schedules"]}


def requester_secret():
    with tempfile.TemporaryDirectory(prefix="catalog-member-") as directory:
        Path(directory, "key-seed").symlink_to("/root/.internkim/secrets/buzz-key-seed")
        subprocess.run([str(workspace / "host/buzz/my-buzz-key"), requester_email], env={**os.environ, "BUZZ_STATE_DIRECTORY": directory}, check=True, capture_output=True)
        written = list(Path(directory).glob("buzz-key-*.txt"))
        assert len(written) == 1, "my-buzz-key must write exactly one key"
        return written[0].read_text().strip()


def send_direct_message(secret, text):
    sent_at = datetime.now(timezone.utc)
    return sent_at, request("/v1/platform/buzz/dm.send", {"userSecretHex": secret, "message": text}, origin=chatd_origin)


def settled_run_after(person_id, channel_id, sent_at, name):
    for listed in request("/admin/api/run?viewerIsAdmin=true"):
        if listed["requesterPersonID"] != person_id or listed["status"] in {"planned", "running"}:
            continue
        if listed.get("originConversationID") != channel_id or datetime.fromisoformat(listed["createdAt"]) < sent_at:
            continue
        detail = request("/admin/api/run/detail?" + urlencode({"taskRunID": listed["taskRunID"], "viewerIsAdmin": "true"}))
        return save("run-" + name, detail)
    return None


def requested_tools(detail):
    return [event["name"] for event in detail["taskEvents"] if event["name"].startswith("tool.") and event["name"].endswith(".requested")]


def main():
    policy = request("/admin/api/policy")
    people = [person for person in policy["people"] if requester_email in person["emails"]]
    assert len(people) == 1, "the isolated seed account must resolve uniquely"
    person_id = people[0]["personID"]
    before = set(save("schedules-before", schedules_of(person_id)))
    secret = requester_secret()
    channel_id = save("direct-message", send_direct_message(secret, "/stop")[1])["channelID"]
    try:
        sent_at, _ = send_direct_message(secret, create_request)
        created_run = wait_for(lambda: settled_run_after(person_id, channel_id, sent_at, "create"), "the create request to settle", 600)
        assert created_run["taskRun"]["status"] == "completed", f"create: the run ended {created_run['taskRun']['status']}"
        assert "tool.schedule_create.requested" in requested_tools(created_run), f"create: no schedule_create call among {requested_tools(created_run)}"
        created = {schedule_id: item for schedule_id, item in schedules_of(person_id).items() if schedule_id not in before}
        save("schedules-created", created)
        assert len(created) == 1, f"create: expected one new schedule, found {len(created)}"
        schedule = next(iter(created.values()))
        assert schedule["platform"] == "buzz" and schedule["conversationID"] == channel_id, f"create: the schedule is not bound to the direct message: {schedule}"
        assert schedule["replyTargetID"], "create: the schedule has no reply target"

        sent_at, _ = send_direct_message(secret, cancel_request)
        cancelled_run = wait_for(lambda: settled_run_after(person_id, channel_id, sent_at, "cancel"), "the cancel request to settle", 600)
        assert cancelled_run["taskRun"]["status"] == "completed", f"cancel: the run ended {cancelled_run['taskRun']['status']}"
        assert "tool.schedule_cancel.requested" in requested_tools(cancelled_run), f"cancel: no schedule_cancel call among {requested_tools(cancelled_run)}"
        after = save("schedules-after", schedules_of(person_id))
        assert after[schedule["taskScheduleID"]]["nextRunAt"] is None, f"cancel: the schedule still has a next run: {after[schedule['taskScheduleID']]}"
        save("revision", request("/admin/api/harness"))
        print("schedule-through-the-catalog: ok", flush=True)
    finally:
        for schedule_id in set(schedules_of(person_id)) - before:
            request("/admin/api/schedule/delete", {"taskScheduleID": schedule_id, "creatorPersonID": person_id}, accepted=(200, 404))


try:
    main()
except Exception as error:
    save("failure", {"error": str(error)})
    raise
