import json
import os
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timedelta
from pathlib import Path
from urllib.parse import urlencode
from zoneinfo import ZoneInfo


evidence = Path(sys.argv[1])
evidence.mkdir(parents=True, exist_ok=True)
workspace = Path("/mnt/shared/workspace")
chatd_origin = "http://172.31.0.1:18090"
requester_email = "member4@example.com"
schedule_write_events = {"tool.schedule_create.requested", "tool.schedule_update.requested", "tool.schedule_cancel.requested"}
firings = {
    "work-only": "도구를 사용하지 말고 173 + 289의 결과를 짧게 답한다.",
    "reads-like-a-request": "매일 이 시간에 주간 보고 알림을 보내줘.",
}


def request(path, body=None, as_requester=False, accepted=(200,), timeout=15, origin=None):
    command = ["curl", "--silent", "--show-error", "--max-time", str(timeout), "--write-out", "\n%{http_code}"]
    if as_requester:
        command += ["--unix-socket", "/run/internkim/admind.sock", "-H", f"X-INTERNKIM-REQUESTER-EMAIL: {requester_email}"]
    if body is not None:
        command += ["-H", "Content-Type: application/json", "-d", "@-"]
    origin = origin or ("http://localhost" if as_requester else "http://127.0.0.1:8080")
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


def statuses_of(schedules):
    return {schedule_id: item.get("status") for schedule_id, item in schedules.items()}


def open_requester_direct_message():
    with tempfile.TemporaryDirectory(prefix="firing-member-") as directory:
        Path(directory, "key-seed").symlink_to("/root/.internkim/secrets/buzz-key-seed")
        subprocess.run([str(workspace / "host/buzz/my-buzz-key"), requester_email], env={**os.environ, "BUZZ_STATE_DIRECTORY": directory}, check=True, capture_output=True)
        written = list(Path(directory).glob("buzz-key-*.txt"))
        assert len(written) == 1, "my-buzz-key must write exactly one key"
        return request("/v1/platform/buzz/dm.send", {"userSecretHex": written[0].read_text().strip(), "message": "/stop"}, origin=chatd_origin)


def create_daily_schedule(name, task_instruction, run_at, time_zone_name, direct_message):
    answer = request("/memory/api/schedules/tool-create", {
        "description": "firing-schedules-nothing " + name,
        "taskInstruction": task_instruction,
        "kind": "cron",
        "cronExpression": f"{run_at.minute} {run_at.hour} * * *",
        "timeZone": time_zone_name,
        "repeatPolicy": "unbounded",
        "platform": "buzz",
        "conversationID": direct_message["channelID"],
        "replyTargetID": direct_message["replyTargetID"],
    }, as_requester=True)
    schedule_id = answer.get("scheduleID")
    if not isinstance(schedule_id, str):
        raise RuntimeError(f"schedule_create returned no schedule: {answer}")
    return schedule_id


def launched_from(detail):
    launches = [json.loads(event["body"]) for event in detail["taskEvents"] if event["name"] == "agent.task_launched"]
    return launches[0].get("sourceReference") if launches else None


def fired_run(person_id, schedule_id, name):
    for listed in request("/admin/api/run?viewerIsAdmin=true"):
        if listed["requesterPersonID"] != person_id or listed["status"] in {"planned", "running"}:
            continue
        detail = request("/admin/api/run/detail?" + urlencode({"taskRunID": listed["taskRunID"], "viewerIsAdmin": "true"}))
        if launched_from(detail) == schedule_id:
            return save("run-" + name, detail)
    return None


def main():
    policy = request("/admin/api/policy")
    people = [person for person in policy["people"] if requester_email in person["emails"]]
    assert len(people) == 1, "the isolated seed account must resolve uniquely"
    person_id = people[0]["personID"]
    time_zone_name = policy["company"].get("timeZone") or "Asia/Seoul"
    before = statuses_of(save("schedules-before", schedules_of(person_id)))
    direct_message = save("direct-message", open_requester_direct_message())
    created = {}
    try:
        run_at = datetime.now(ZoneInfo(time_zone_name)).replace(second=0, microsecond=0) + timedelta(minutes=2)
        for name, task_instruction in firings.items():
            created[name] = create_daily_schedule(name, task_instruction, run_at, time_zone_name, direct_message)
        expected = {**before, **statuses_of(schedules_of(person_id))}
        for name, schedule_id in created.items():
            detail = wait_for(lambda: fired_run(person_id, schedule_id, name), f"the {name} schedule to fire and settle", 600)
            assert detail["taskRun"]["status"] == "completed", f"{name}: the fired run ended {detail['taskRun']['status']}"
            written = [event["name"] for event in detail["taskEvents"] if event["name"] in schedule_write_events]
            assert not written, f"{name}: the fired run wrote a schedule: {written}"
        after = statuses_of(save("schedules-after", schedules_of(person_id)))
        assert after == expected, f"firing changed the schedules: expected {expected}, found {after}"
        save("revision", request("/admin/api/harness"))
        print("firing-schedules-nothing: ok", flush=True)
    finally:
        for schedule_id in set(schedules_of(person_id)) - set(before):
            request("/admin/api/schedule/delete", {"taskScheduleID": schedule_id, "creatorPersonID": person_id}, accepted=(200, 404))


try:
    main()
except Exception as error:
    save("failure", {"error": str(error)})
    raise
