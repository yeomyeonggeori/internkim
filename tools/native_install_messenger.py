#!/usr/bin/env python3
"""What the native-install rig asks of a company host's messenger, from inside the guest.

The same file is imported by the rig, for the judgements it makes about what came back, and
copied into the guest and run there, because chatd, the agent's database and admind's socket
answer only on the guest's own loopback. It needs nothing but the standard library and `psql`.

    native_install_messenger.py <action> '<json arguments>'

Each action prints one JSON document.
"""

import base64
import hashlib
import json
import re
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

CHATD_ENDPOINT = "http://127.0.0.1:18090"
AGENT_ENDPOINT = "http://127.0.0.1:8080"
IDENTITY_SEED_PATH = "/var/lib/internkim/current/secrets/buzz-key-seed"
AGENT_DATABASE_PATH = "/var/lib/internkim/current/host.env"
ADMIND_SOCKET_PATH = "/run/internkim/admind.sock"
AGENT_UNIT_NAME = "blueclaw.service"
PLATFORM_ROUTE = "/v1/platform/buzz/"
PICTURE_NAME = "buzz-attachment-word.png"
WORD_THE_PICTURE_CARRIES = "SALT"
PICTURE_QUESTION = "What word is written in this picture? Answer with the word as written."
REFUSED_STATUSES = ("failed", "cancelled")
FINISHED_STATUSES = ("completed", *REFUSED_STATUSES)


def buzz_secret(seed, email):
    """The key a person's messenger identity is derived from, the formula buzzidentity.Secret states."""
    return hashlib.sha256(f"{seed}|secret|{email.strip().lower()}".encode()).hexdigest()


def actor_of(secret):
    return {"kind": "buzz-secret", "secret": secret}


def policy_path_of(execution_line):
    found = re.search(r"-policy (\S+?)(?:\s|;|$)", execution_line)
    return found.group(1) if found else ""


def people_of(policy):
    return [
        {"personID": person.get("personID", ""), "email": person["emails"][0]}
        for person in policy.get("people") or []
        if person.get("emails")
    ]


def strings_in(document):
    if isinstance(document, str):
        yield document
    elif isinstance(document, dict):
        for held in document.values():
            yield from strings_in(held)
    elif isinstance(document, list):
        for held in document:
            yield from strings_in(held)


def holds_text(document, marker):
    return any(marker in text for text in strings_in(document))


def tool_reads_in(event_bodies):
    reads = []
    for body in event_bodies:
        try:
            decoded = json.loads(body) if isinstance(body, str) else body
        except ValueError:
            continue
        if isinstance(decoded, dict) and decoded.get("tool") == "read":
            reads.append(decoded)
    return reads


def reads_that_found_nothing(event_bodies):
    return [read for read in tool_reads_in(event_bodies) if (read.get("failure") or {}).get("code") == "not_found"]


def is_a_hex_identifier(text):
    return bool(re.fullmatch(r"[0-9a-f]{8,128}", text or ""))


def post_json(url, document):
    request = urllib.request.Request(
        url, data=json.dumps(document).encode(), headers={"Content-Type": "application/json"}, method="POST"
    )
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            return json.loads(response.read() or b"{}")
    except urllib.error.HTTPError as refusal:
        raise SystemExit(f"{url} answered {refusal.code}: {refusal.read().decode(errors='replace')[:400]}")


def ask_chatd(route, document):
    return post_json(CHATD_ENDPOINT + PLATFORM_ROUTE + route, document)


def read_text(path):
    return Path(path).read_text().strip()


def secret_of(email):
    return buzz_secret(read_text(IDENTITY_SEED_PATH), email)


def agent_database_url():
    for line in Path(AGENT_DATABASE_PATH).read_text().splitlines():
        if line.startswith("DATABASE_URL="):
            return line.split("=", 1)[1].strip()
    raise SystemExit(f"no DATABASE_URL in {AGENT_DATABASE_PATH}")


def query_agent_database(statement, **variables):
    arguments = ["psql", agent_database_url(), "--quiet", "--tuples-only", "--no-align", "--set", "ON_ERROR_STOP=1"]
    for name, value in variables.items():
        arguments += ["--set", f"{name}={value}"]
    completed = subprocess.run(arguments, input=statement, capture_output=True, text=True, timeout=60)
    if completed.returncode != 0:
        raise SystemExit(f"psql failed: {completed.stderr.strip()[:400]}")
    return completed.stdout.strip()


def act_people(_):
    shown = subprocess.run(
        ["systemctl", "show", AGENT_UNIT_NAME, "-p", "ExecStart", "--value"], capture_output=True, text=True
    ).stdout
    path = policy_path_of(shown)
    if not path:
        return {"people": [], "problem": f"{AGENT_UNIT_NAME} names no -policy"}
    return {"people": people_of(json.loads(Path(path).read_text())), "policyPath": path}


def act_identity(arguments):
    return ask_chatd("person.identity", {"actor": actor_of(secret_of(arguments["email"]))})


def act_send_picture(arguments):
    secret = secret_of(arguments["email"])
    actor = actor_of(secret)
    conversation = ask_chatd("person.dm.ensure", {"actor": actor, "counterpartExternalIDs": []})
    contents = Path(arguments["picturePath"]).read_bytes()
    sent = ask_chatd(
        "person.message.send",
        {
            "actor": actor,
            "conversationID": conversation["id"],
            "body": arguments["body"],
            "attachments": [
                {
                    "filename": arguments["filename"],
                    "contentType": "image/png",
                    "contentBase64": base64.b64encode(contents).decode(),
                }
            ],
        },
    )
    return {"conversationID": conversation["id"], "messageID": sent.get("id", ""), "sha256": hashlib.sha256(contents).hexdigest()}


LEDGER_STATEMENT = """
select coalesce(row_to_json(found), '{}'::json) from (
  select run.task_run_id as "taskRunID", run.requester_person_id as "requesterPersonID", run.status, run.result,
         (select coalesce(json_agg(event.body), '[]'::json) from task_event event where event.task_run_id = run.task_run_id) as events
  from task_run run
  where position(:'message' in coalesce(run.origin_reply_target_id, '')) > 0
  order by run.created_at desc limit 1
) found;
"""


def act_ledger(arguments):
    if not is_a_hex_identifier(arguments["messageID"]):
        raise SystemExit("messageID is not a hex identifier")
    found = query_agent_database(LEDGER_STATEMENT, message=arguments["messageID"])
    return {"run": json.loads(found) or None}


def act_find_file(arguments):
    located = subprocess.run(
        ["find", "/workspace", "-type", "f", "-name", arguments["filename"]], capture_output=True, text=True
    ).stdout.split()
    if not located:
        return {"file": None}
    path = located[0]
    owner = subprocess.run(["stat", "-c", "%U", path], capture_output=True, text=True).stdout.strip()
    return {"file": {"path": path, "owner": owner, "sha256": hashlib.sha256(Path(path).read_bytes()).hexdigest()}}


def act_invoke_direct_message(arguments):
    body = json.dumps({"input": {"targetType": "directMessage", "personHint": arguments["recipientEmail"], "message": arguments["message"]}})
    completed = subprocess.run(
        [
            "curl", "-sS", "--max-time", "120", "--unix-socket", ADMIND_SOCKET_PATH,
            "-X", "POST", "http://internkim/api/v1/tools/message_send/invoke",
            "-H", "Content-Type: application/json",
            "-H", f"X-INTERNKIM-REQUESTER-EMAIL: {arguments['requesterEmail']}",
            "-H", "X-INTERNKIM-REQUESTER-PERMISSION: write",
            "-w", "\nstatus=%{http_code}\n", "--data", body,
        ],
        capture_output=True, text=True,
    )
    lines = completed.stdout.strip().splitlines()
    status = lines[-1].split("=", 1)[1] if lines and lines[-1].startswith("status=") else ""
    try:
        document = json.loads("\n".join(lines[:-1]))
    except ValueError:
        document = {"unreadable": completed.stdout[:300], "stderr": completed.stderr[:300]}
    return {"status": status, "answer": document}


def act_read_inbox(arguments):
    recipient = actor_of(secret_of(arguments["recipientEmail"]))
    sender = ask_chatd("person.identity", {"actor": actor_of(secret_of(arguments["senderEmail"]))})
    conversation = ask_chatd("person.dm.ensure", {"actor": recipient, "counterpartExternalIDs": [sender["externalID"]]})
    messages = ask_chatd("person.messages.list", {"actor": recipient, "conversationID": conversation["id"]})
    return {"isDelivered": holds_text(messages, arguments["marker"]), "conversationID": conversation["id"]}


def act_open_mention(arguments):
    actor = actor_of(secret_of(arguments["email"]))
    agent = ask_chatd("identity.self", {})["pubkeyHex"]
    channel = ask_chatd(
        "person.channel.create",
        {"actor": actor, "name": arguments["channelName"], "visibility": "private", "memberExternalIDs": [agent]},
    )
    sent = ask_chatd(
        "person.message.send",
        {
            "actor": actor,
            "conversationID": channel["id"],
            "body": arguments["body"],
            "mentions": {"externalIDs": [agent]},
        },
    )
    return {"channelID": channel["id"], "messageID": sent.get("id", ""), "agentPubkey": agent, "uninvited": channel.get("uninvitedExternalIDs") or []}


def act_connector_events(arguments):
    if not is_a_hex_identifier(arguments["messageID"]):
        raise SystemExit("messageID is not a hex identifier")
    url = f"{AGENT_ENDPOINT}/admin/api/connector/events?platform=buzz&messageID={arguments['messageID']}&limit=5"
    with urllib.request.urlopen(url, timeout=30) as response:
        events = json.loads(response.read())
    return {"isRecorded": any(event.get("externalMessageID") == arguments["messageID"] for event in events), "events": events[:3]}


ACTIONS = {
    "people": act_people,
    "identity": act_identity,
    "send_picture": act_send_picture,
    "ledger": act_ledger,
    "find_file": act_find_file,
    "invoke_direct_message": act_invoke_direct_message,
    "read_inbox": act_read_inbox,
    "open_mention": act_open_mention,
    "connector_events": act_connector_events,
}


def main(argv):
    if len(argv) < 2 or argv[1] not in ACTIONS:
        raise SystemExit(f"usage: {argv[0]} <{'|'.join(ACTIONS)}> '<json arguments>'")
    arguments = json.loads(argv[2]) if len(argv) > 2 else {}
    print(json.dumps(ACTIONS[argv[1]](arguments)))


if __name__ == "__main__":
    main(sys.argv)
