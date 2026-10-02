"""What the rig needs to ask a host's admin gateway for an update the way host_update does."""

import json
import re
import shlex
from pathlib import Path

REPOSITORY_ROOT = Path(__file__).resolve().parent.parent
ADMIND_HOST_UPDATE_SOURCE = REPOSITORY_ROOT / "internal" / "admind" / "host_update.go"

UPDATE_UNIT_NAME = "internkim-host-update.service"
ADMIND_SOCKET_PATH = "/run/internkim/admind.sock"
NOTE_PATH = "/var/lib/internkim/host-update.json"
CHANNEL_RECORD_PATH = "/var/lib/internkim/release-channel"
MESSENGER_PLATFORM = "buzz"
ADMIND_DROP_IN_PATH = "/etc/systemd/system/internkim-admind.service.d/rig-release-source.conf"
UPDATE_UNIT_DROP_IN_PATH = "/etc/systemd/system/internkim-host-update.service.d/rig-release-source.conf"
MONITOR_UNIT_NAME = "rig-downtime-monitor"
MONITOR_LOG_PATH = "/var/tmp/rig-downtime.log"
RELEASE_API_PATH = "api"
OUTCOME_PROMPT_FRAGMENT = "Tell the requester how the update of this company"
VERSION_PATH = "/host/api/version"
PLAN_PATH = "/host/api/update/plan"
START_PATH = "/host/api/update"


def promised_downtime_seconds():
    """The figure admind puts in front of an administrator, read where admind keeps it."""
    match = re.search(r"expectedHostUpdateDowntimeSeconds\s*=\s*(\d+)", ADMIND_HOST_UPDATE_SOURCE.read_text())
    if match is None:
        raise AssertionError("internal/admind/host_update.go no longer declares expectedHostUpdateDowntimeSeconds")
    return int(match.group(1))


def stable_release(tag, published_at):
    return {"tag_name": tag, "published_at": published_at, "body": f"Release {tag}, built by the native install rig.", "prerelease": False, "draft": False}


def write_stable_listing(release_root, tags_newest_first, published_at):
    """GitHub's release list, served from the rig's release directory as the directory's index.html."""
    releases = [stable_release(tag, published_at) for tag in tags_newest_first]
    directory = Path(release_root) / RELEASE_API_PATH / "releases"
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "index.html").write_text(json.dumps(releases))


def release_source_drop_ins(api_url, download_url):
    """Where the gateway reads the release list, and where the update unit's install.sh downloads from."""
    return {
        ADMIND_DROP_IN_PATH: f"[Service]\nEnvironment=INTERNKIM_RELEASE_API_URL={api_url}\n",
        UPDATE_UNIT_DROP_IN_PATH: f"[Service]\nEnvironment=INTERNKIM_INSTALL_RELEASE_URL={download_url}\n",
    }


def ask_admind_command(path, email, body):
    return (
        f"curl -sS --max-time 60 --unix-socket {ADMIND_SOCKET_PATH} "
        f"-H {shlex.quote('X-INTERNKIM-REQUESTER-EMAIL: ' + email)} -H 'Content-Type: application/json' "
        f"-w '\\nstatus=%{{http_code}}\\n' --data {shlex.quote(json.dumps(body))} http://internkim{path}"
    )


def answer_of(reading):
    """The JSON body and the status a curl from ask_admind_command printed."""
    lines = reading.strip().splitlines()
    status = lines[-1].split("=", 1)[1] if lines and lines[-1].startswith("status=") else ""
    body = "\n".join(lines[:-1]) if status else reading
    try:
        document = json.loads(body)
    except ValueError:
        document = {}
    return status, document


def monitor_command(probes):
    checks = " ".join(
        f'[ "$(curl -s -o /dev/null -w %{{http_code}} --max-time 2 http://127.0.0.1:{port}{path})" = 200 ] || state=down;'
        for port, path in probes
    )
    loop = (
        "while :; do state=up; "
        "systemctl is-active --quiet internkim-relay.service || state=down; "
        f"{checks} "
        f'echo "$(date +%s.%N) $state" >> {MONITOR_LOG_PATH}; sleep 1; done'
    )
    return (
        f"rm -f {MONITOR_LOG_PATH}; "
        f"systemd-run --unit={MONITOR_UNIT_NAME} --collect /bin/sh -c {shlex.quote(loop)} >/dev/null 2>&1; "
        f"systemctl is-active {MONITOR_UNIT_NAME}"
    )


def downtime_from(monitor_log):
    """The longest run of readings in which any service was not answering, in seconds."""
    readings = []
    for line in monitor_log.splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1] in ("up", "down"):
            readings.append((float(parts[0]), parts[1]))
    longest = 0.0
    went_down = None
    for moment, state in readings:
        if state == "down" and went_down is None:
            went_down = moment
        if state == "up" and went_down is not None:
            longest = max(longest, moment - went_down)
            went_down = None
    is_back = went_down is None
    return longest, is_back, len(readings)


ORIGIN_COMMAND = r"""
agent_url="$(sed -n 's/^DATABASE_URL=//p' %(agent_path)s 2>/dev/null | head -1)"
psql "$agent_url" -tAF '|' -c "select coalesce(requester_person_id,''), coalesce(origin_conversation_id,''), coalesce(origin_reply_target_id,'') from task_run where position('%(text)s' in prompt) > 0 order by created_at desc limit 1" 2>&1
"""

OUTCOME_COMMAND = r"""
agent_url="$(sed -n 's/^DATABASE_URL=//p' %(agent_path)s 2>/dev/null | head -1)"
messenger_url="$(sed -n 's/^DATABASE_URL=//p' %(messenger_path)s 2>/dev/null | head -1)"
created="$(psql "$agent_url" -tAc "select extract(epoch from created_at)::bigint from task_run where position('%(fragment)s' in prompt) > 0 order by created_at desc limit 1" 2>&1 | tr -d ' \n')"
printf 'task=%%s\n' "$(psql "$agent_url" -tAc "select status from task_run where position('%(fragment)s' in prompt) > 0 order by created_at desc limit 1" 2>&1 | tr -d ' \n')"
printf 'answers=%%s\n' "$(psql "$messenger_url" -tAc "select count(*) from events answer join events asked on encode(asked.id,'hex') = '%(member_message)s' where answer.kind = 9 and answer.channel_id = asked.channel_id and answer.pubkey <> asked.pubkey and answer.created_at >= to_timestamp(${created:-0})" 2>&1 | tr -d ' \n')"
printf 'said=%%s\n' "$(psql "$messenger_url" -tAc "select replace(answer.content, chr(10), ' ') from events answer join events asked on encode(asked.id,'hex') = '%(member_message)s' where answer.kind = 9 and answer.channel_id = asked.channel_id and answer.pubkey <> asked.pubkey and answer.created_at >= to_timestamp(${created:-0}) order by answer.created_at desc limit 1" 2>&1)"
"""
