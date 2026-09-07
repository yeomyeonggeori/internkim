import hashlib
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
requester_email = "member1@example.com"
workspace = Path("/mnt/shared/workspace")


def request(path, body=None, persona=False, accepted=(200,), origin=None):
    command = ["curl", "--silent", "--show-error", "--max-time", "15", "--write-out", "\n%{http_code}"]
    if persona:
        command += ["--unix-socket", "/run/internkim/admind.sock", "-H", f"X-INTERNKIM-REQUESTER-EMAIL: {requester_email}"]
    if body is not None:
        command += ["-H", "Content-Type: application/json", "-d", "@-"]
    origin = origin or ("http://localhost" if persona else "http://127.0.0.1:8080")
    response = subprocess.run(command + [origin + path], input=json.dumps(body) if body is not None else None, capture_output=True, text=True, check=True)
    payload, status = response.stdout.rsplit("\n", 1)
    if int(status) not in accepted:
        raise RuntimeError(f"{path}: HTTP {status}: {payload}")
    if int(status) != 200:
        return {"status": int(status), "body": payload}
    return json.loads(payload)


def save(name, value):
    (evidence / (name + ".json")).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    return value


def wait_for(read, description, timeout=100):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = read()
        if value:
            return value
        time.sleep(2)
    raise RuntimeError(f"timed out waiting for {description}")


def run_acceptance_binary(name, command, directory, environment=None):
    log = evidence / (name + ".log")
    with log.open("w") as output:
        subprocess.run(command, cwd=directory, env=environment, stdout=output, stderr=subprocess.STDOUT, check=True, timeout=360)
    assert any(line.startswith("--- PASS: " + name + " (") for line in log.read_text().splitlines()), "acceptance binary skipped its test"


def verify_database_and_model():
    for provider_test in ["TestAutoProviderDoesNotFallBackToLocalAfterRemoteFailure", "TestStructuredLLMFailurePersistsProviderExchanges"]:
        run_acceptance_binary(provider_test, [str(workspace / "build/briefing-provider.test"), "-test.v", "-test.run=^" + provider_test + "$"], workspace)
    database_test = "TestReconcileMorningBriefingsPersistsLifecycleAndGuardsGenericMutations"
    run_acceptance_binary(database_test, ["sudo", "-u", "postgres", "env", "BLUECLAW_TEST_POSTGRES_URL=postgresql://postgres@/postgres?host=/var/run/postgresql&sslmode=disable", str(workspace / "build/morning-briefing-postgres.test"), "-test.v", "-test.run=^" + database_test + "$"], workspace / ".dependency/blueclaw/internal/store/postgres")
    environment = {**os.environ, "BLUECLAW_E2E_LIVE": "1", "BLUECLAW_E2E_LLM_UNIX_SOCKET": "/run/internkim/capability.sock", "BLUECLAW_SCENARIO_CAPABILITY_CATALOG": str(workspace / "pkg/capabilityprotocol/generated/capability-tools.json"), "BLUECLAW_SCENARIO_SKILL_ROOTS": str(workspace / ".dependency/internkim-plugin/skills")}
    model_test = "TestMorningBriefingSettingsLive"
    run_acceptance_binary(model_test, [str(workspace / "build/morning-briefing-live.test"), "-test.v", "-test.run=^" + model_test + "$"], workspace / ".dependency/blueclaw/internal/e2e", environment)


def connect_test_messenger():
    with tempfile.TemporaryDirectory(prefix="briefing-member-") as directory:
        Path(directory, "key-seed").symlink_to("/root/.internkim/secrets/buzz-key-seed")
        subprocess.run([str(workspace / "host/buzz/my-buzz-key"), requester_email], env={**os.environ, "BUZZ_STATE_DIRECTORY": directory}, check=True, capture_output=True)
        keys = list(Path(directory).glob("buzz-key-*.txt"))
        assert len(keys) == 1
        return request("/v1/platform/buzz/dm.send", {"userSecretHex": keys[0].read_text().strip(), "message": "/stop"}, origin="http://172.31.0.1:18090")


def main():
    if len(sys.argv) < 3 or sys.argv[2] != "scheduled":
        verify_database_and_model()
    policy = save("policy", request("/admin/api/policy"))
    if not policy["company"].get("timeZone"):
        policy["company"]["timeZone"] = "Asia/Seoul"
        policy_path = Path("/var/lib/blueclaw/delivery/config/policy.json")
        with policy_path.open("w") as output:
            json.dump(policy, output, ensure_ascii=False)
            output.flush()
            os.fsync(output.fileno())
        wait_for(lambda: request("/admin/api/policy").get("company", {}).get("timeZone") == policy["company"]["timeZone"], "guest-visible company timezone")
        policy = save("policy-configured", request("/admin/api/policy"))
    save("policy-reloaded", request("/admin/api/policy/reload", {}))
    people = [person for person in policy["people"] if requester_email in person["emails"]]
    assert len(people) == 1, "the isolated seed account must resolve uniquely"
    person_id = people[0]["personID"]
    schedule_id = hashlib.sha256(("morning-briefing:" + person_id).encode()).hexdigest()
    timezone = ZoneInfo(policy["company"]["timeZone"])
    profile = save("profile-before", request("/persona/api/user", persona=True))
    assert profile["morningBriefing"] == {"enabled": True, "time": "08:00"}
    profile["language"] = {"default": "ko"}
    save("messenger-connected", connect_test_messenger())

    def schedule_matching(predicate=lambda value: True):
        query = urlencode({"creatorPersonID": person_id, "includeExpired": "true", "pageSize": 200})
        response = request("/admin/api/schedule?" + query)
        matches = [item for item in response["schedules"] if item["taskScheduleID"] == schedule_id]
        assert len(matches) <= 1, "duplicate managed morning briefings"
        save("schedule-latest", response)
        return matches[0] if matches and predicate(matches[0]) else None

    save("schedule-before", wait_for(schedule_matching, "default morning briefing"))
    profile["morningBriefing"] = {"enabled": False, "time": "09:15"}
    save("profile-disabled", request("/persona/api/user", profile, persona=True))
    save("schedule-disabled", wait_for(lambda: schedule_matching(lambda item: item.get("nextRunAt") is None), "disabled morning briefing"))
    save("delete-refused", request("/admin/api/schedule/delete", {"taskScheduleID": schedule_id, "creatorPersonID": person_id}, accepted=(404,)))
    assert schedule_matching(), "generic deletion removed the built-in hook"

    run_at = datetime.now(timezone).replace(second=0, microsecond=0) + timedelta(minutes=2)
    profile["morningBriefing"] = {"enabled": True, "time": run_at.strftime("%H:%M")}
    save("profile-enabled", request("/persona/api/user", profile, persona=True))
    enabled = wait_for(lambda: schedule_matching(lambda item: item.get("nextRunAt") is not None), "reenabled morning briefing")
    assert datetime.fromisoformat(enabled["nextRunAt"].replace("Z", "+00:00")) == run_at
    save("schedule-enabled", enabled)

    def delivered():
        current = schedule_matching()
        if not current or not current.get("lastTaskRunID"):
            return None
        detail = save("run-latest", request("/admin/api/run/detail?" + urlencode({"taskRunID": current["lastTaskRunID"]})))
        assert detail["taskRun"]["status"] == "completed", "morning task did not complete"
        events = detail["taskEvents"]
        sent = [event for event in events if event["name"] == "connector.reply.sent"]
        if not sent:
            return None
        assert len(sent) == 1, "duplicate private briefing delivery"
        names = {event["name"] for event in events}
        contexts = [json.loads(event["body"]) for event in events if event["name"] == "task.model_visible_context"]
        assert contexts and contexts[0]["responseLanguage"] == "ko", "scheduled run lost the requester's profile language"
        assert "tool.task_list.requested" in names, "briefing did not query live work"
        assert "tool.event_list.requested" in names, "briefing did not query live calendar"
        assert "tool.message_send.requested" not in names, "briefing sent a second message itself"
        return detail

    save("run-delivered", wait_for(delivered, "real scheduled briefing and private delivery", 420))
    profile["morningBriefing"]["enabled"] = False
    save("profile-cleanup", request("/persona/api/user", profile, persona=True))
    save("health", request("/admin/api/health"))
    save("revision", request("/admin/api/harness"))
    print("morning-briefing: ok", flush=True)


try:
    main()
except Exception as error:
    save("failure", {"error": str(error)})
    raise
