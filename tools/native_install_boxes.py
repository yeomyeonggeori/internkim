"""What `tools/test-native-install --boxes` observes of a company on three boxes, and how a run is judged.

The scenario names every observation it makes in OBSERVATIONS. NOT_YET names the ones that fail on
main today, each with the pull request expected to make it hold. A run passes only when the
observations that did not hold are exactly NOT_YET: an unlisted failure fails the run, and so does a
listed observation that holds, so the list only shrinks. An observation the run never reached did
not hold.
"""

import base64
import datetime
import grp
import hashlib
import http.client
import http.server
import json
import os
import pwd
import re
import socket
import sqlite3
import stat
import statistics
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
from dataclasses import dataclass
from pathlib import Path

INSTALLED_ON_A = "the published one-liner leaves the package installed"
INSTALLED_ON_B = "the install line installs the package on box B"
INSTALLED_ON_C = "the install line installs the package on box C"
A_CLAIMED = "box A is claimed by the code it shows and serves the company"
B_ADDED = "box B is added by the code it shows"
C_ADDED = "box C is added by the code it shows"
A_SERVES = "box A runs every company service and holds the gateway socket"
STANDBYS_IDLE = "boxes B and C stand by with only the box daemon awake"
SETUP_SAYS_READY = "the setup page says every box is ready"
SHARES_REPORTED = "each standby reports half the company's pieces as its share"
SHARES_ON_DISK = "the standbys hold every piece the store holds on disk, each piece on one standby"

AGENT_SETTLED = "the agent settles before the state is read"
WRITERS_ACKNOWLEDGED = "every writer is acknowledged before the power goes"
AUTOMATIC_TAKE_OVER = "a standby takes the company over by itself within five minutes of the power going"
TAKEN_OVER_RUNS = "the box that took over runs every company service"
COMMITS_KEPT = "every acknowledged database commit is on the box that took over"
FILE_WRITES_KEPT = "every acknowledged file write is on the box that took over"
MESSAGES_KEPT = "every acknowledged message is in the messenger of the box that took over"
FILES_EQUAL = "the files off the write paths are what the service box held before the power went"
ACCOUNTS_EQUAL = "the accounts, groups and memberships are what they were"
SCHEDULES_EQUAL = "the scheduled tasks are what they were"
ROSTER_EQUAL = "the roster is what it was"
SECRETS_EQUAL = "the model key and the mail credentials are what they were"
WALLS_HOLD = "on the box that took over each person reads their own files and nobody else's"
SCHEDULE_RUNS = "the task scheduled for just after the state was read runs on the box that took over"

MOVE_INTERRUPTS_BRIEFLY = "moving the company to another box keeps members waiting under a minute"
COMMITS_MOVED = "every acknowledged database commit is on the box the company moved to"
FILE_WRITES_MOVED = "every acknowledged file write is on the box the company moved to"
MESSAGES_MOVED = "every acknowledged message is in the messenger of the box the company moved to"
OLD_BOX_COMMITS_REFUSED = "the box the company moved from has its database commits refused"
OLD_BOX_STORE_WRITES_REFUSED = "the box the company moved from has its writes to the store refused"

STORE_LINK_LIMITED = "every box reaches the store through a link limited to 100 Mbit"
HALF_HELD_IS_FASTER = "a standby holding its share takes over sooner than an empty box restoring from the store"

OBSERVATIONS = (
    INSTALLED_ON_A,
    INSTALLED_ON_B,
    INSTALLED_ON_C,
    A_CLAIMED,
    B_ADDED,
    C_ADDED,
    A_SERVES,
    STANDBYS_IDLE,
    SETUP_SAYS_READY,
    SHARES_REPORTED,
    SHARES_ON_DISK,
    AGENT_SETTLED,
    WRITERS_ACKNOWLEDGED,
    AUTOMATIC_TAKE_OVER,
    TAKEN_OVER_RUNS,
    COMMITS_KEPT,
    FILE_WRITES_KEPT,
    MESSAGES_KEPT,
    FILES_EQUAL,
    ACCOUNTS_EQUAL,
    SCHEDULES_EQUAL,
    ROSTER_EQUAL,
    SECRETS_EQUAL,
    WALLS_HOLD,
    SCHEDULE_RUNS,
    MOVE_INTERRUPTS_BRIEFLY,
    COMMITS_MOVED,
    FILE_WRITES_MOVED,
    MESSAGES_MOVED,
    OLD_BOX_COMMITS_REFUSED,
    OLD_BOX_STORE_WRITES_REFUSED,
    STORE_LINK_LIMITED,
    HALF_HELD_IS_FASTER,
)

NOT_YET = {
    OLD_BOX_COMMITS_REFUSED: (
        "pass condition 4, fencing: on the box the company moved from, an insert as its services' role through the "
        "database URL they held is neither refused nor answered within 30 s; its PostgreSQL still takes the "
        "connection and the commit waits"
    ),
    HALF_HELD_IS_FASTER: (
        "seeding step 4: a standby keeps a few pieces of the log on disk after the fill at most, and none in some runs, "
        "so the standby that takes over restores nearly everything from the store as the empty box does, and is no sooner"
    ),
}


@dataclass(frozen=True)
class Ratchet:
    held: tuple
    not_yet: tuple
    unlisted_failures: tuple
    listed_but_held: tuple
    unnamed: tuple

    @property
    def passes(self):
        return not (self.unlisted_failures or self.listed_but_held or self.unnamed)


def judge(recorded, observations=OBSERVATIONS, not_yet=None):
    """The ratchet's verdict on what one run recorded: each observation's name and whether it held."""
    expected_failures = NOT_YET if not_yet is None else not_yet
    held = {name for name, passed in recorded.items() if passed}
    failed = [name for name in observations if name not in held]
    return Ratchet(
        held=tuple(name for name in observations if name in held),
        not_yet=tuple(name for name in failed if name in expected_failures),
        unlisted_failures=tuple(name for name in failed if name not in expected_failures),
        listed_but_held=tuple(name for name in observations if name in held and name in expected_failures),
        unnamed=tuple(sorted(name for name in recorded if name not in observations)),
    )


def ratchet_report(ratchet, not_yet=None):
    expected_failures = NOT_YET if not_yet is None else not_yet
    lines = [f"{len(ratchet.held)} hold, {len(ratchet.not_yet)} not yet"]
    lines += [f"    not yet: {name} ({expected_failures[name]})" for name in ratchet.not_yet]
    lines += [f"    failed and not listed as not yet: {name}" for name in ratchet.unlisted_failures]
    lines += [f"    held and still listed as not yet, so take it off NOT_YET: {name}" for name in ratchet.listed_but_held]
    lines += [f"    recorded under a name OBSERVATIONS does not hold: {name}" for name in ratchet.unnamed]
    return "\n".join(lines)



def sequence_kept(rows, acknowledged):
    """Whether the rows a database writer left are a gap-free prefix holding every acknowledged commit.

    One commit may be in flight when the power goes: committed and not yet acknowledged. So the
    prefix may run one past the newest acknowledgement, and no further.
    """
    held = sorted(set(rows))
    newest_acknowledged = max(acknowledged, default=0)
    newest_held = held[-1] if held else 0
    is_a_prefix = held == list(range(1, newest_held + 1))
    is_kept = is_a_prefix and newest_acknowledged <= newest_held <= newest_acknowledged + 1
    return is_kept, (
        f"{len(acknowledged)} acknowledged, newest {newest_acknowledged}; {len(held)} rows, newest {newest_held}; "
        f"gap-free from 1: {is_a_prefix}"
    )


def file_content(index):
    return f"rig file write {index}\n" + hashlib.sha256(str(index).encode()).hexdigest() + "\n"


def file_writes_kept(found, acknowledged):
    """Whether every acknowledged file is there with the content its number gives, with at most one more."""
    wrong = sorted(index for index, content in found.items() if content != file_content(index))
    missing = sorted(set(acknowledged) - set(found))
    unacknowledged = sorted(set(found) - set(acknowledged))
    newest_acknowledged = max(acknowledged, default=0)
    is_kept = not wrong and not missing and set(unacknowledged) <= {newest_acknowledged + 1}
    return is_kept, (
        f"{len(acknowledged)} acknowledged, {len(found)} found; missing {missing[:20]}; "
        f"wrong content {wrong[:20]}; found and not acknowledged {unacknowledged[:20]}"
    )


def messages_kept(stored_identifiers, acknowledged_identifiers):
    missing = sorted(set(acknowledged_identifiers) - set(stored_identifiers))
    return not missing, f"{len(acknowledged_identifiers)} acknowledged, {len(missing)} missing: {missing[:20]}"



LOG_FOLDERS = ("base/", "base-chunks/", "wal/", "wal-history/", "file-journal/", "file-chunks/")
SHARE_TOLERANCE = 0.1


def pieces_of_the_log(stored_paths, company_id):
    """The store's pieces of the company's two logs, by their path under the company's folder."""
    prefix = company_id + "/"
    relative = (path.removeprefix(prefix) for path in stored_paths if path.startswith(prefix))
    return sorted(path for path in relative if path.startswith(LOG_FOLDERS))


def shares_split(pieces, held_by_standby):
    """Whether the standbys hold every piece between them, each piece on exactly one."""
    union = set().union(*held_by_standby.values()) if held_by_standby else set()
    overlapping = sorted(
        piece for piece in union if sum(piece in held for held in held_by_standby.values()) > 1
    )
    uncovered = sorted(set(pieces) - union)
    is_split = bool(pieces) and not uncovered and not overlapping
    counts = ", ".join(f"{name} {len(held)}" for name, held in sorted(held_by_standby.items()))
    return is_split, (
        f"{len(pieces)} pieces in the store; held: {counts or 'nothing'}; on no standby {len(uncovered)} "
        f"{uncovered[:10]}; on more than one {len(overlapping)} {overlapping[:10]}"
    )


def shares_reported_evenly(reported):
    """Whether each standby reports a share of the pieces within ten points of an even split."""
    if not reported:
        return False, "no standby reported a share"
    even = 1 / len(reported)
    is_even = all(
        isinstance(share, (int, float)) and abs(share - even) <= SHARE_TOLERANCE for share in reported.values()
    )
    return is_even, f"even share {even:.2f}; reported {json.dumps(reported)}"



SESSION_KEY_NAME = "agent-key"
WRITE_PATH_PREFIX = "rig-writes-"

EXEMPT = {
    ("state", "box"): "each box keeps its own identity, pairing state and session under box/",
    ("state", "media/.vgwlocks"): (
        "the media store's lock files, empty and made and removed as objects are written: a lock is not data, and "
        "hostFileRoots in internal/companyhost/backup.go leaves them out of every backup and every journal"
    ),
    ("state", ".file-journal-barrier"): (
        "the file barrier's markers, each empty and removed once the journal holds the writes before it "
        "(internal/filestream/barrier.go): a marker is not data, and JournaledFileRoots in "
        "internal/companyhost/file_journal_roots.go leaves them out of the journal"
    ),
}
RELAY_PROBE_WRITER = "relay-probe"
RELAY_PROBE_NONCE = rb"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}"
RELAY_PROBE_BODY = re.compile(
    rb"probe-(?:sequential|inm-race)-" + RELAY_PROBE_NONCE + rb"-[0-9]+"
    rb"|probe-etag-[0-9]+-" + RELAY_PROBE_NONCE
    + rb"|probe-pointer-seed"
    rb"|round=[0-9]+,racer=[0-9]+,nonce=" + RELAY_PROBE_NONCE
)
RELAY_PROBE_LARGEST_BYTES = 128
ADDED_WHEN_A_BOX_STARTS = (
    "an object the relay's store probe writes each time the relay starts, known by the body run_conformance_probe "
    "writes (buzz-relay/src/crates/buzz-relay/src/api/git/store.rs), is not counted when the box that took over "
    "added it; one that was there before the power went is still compared"
)
EXEMPT_SESSION_KEYS = (
    "the serving box's session key, in agent-key and as connection.json's agentKey, which the take-over "
    "writes for the box that serves"
)


def is_under(role, path, places):
    return any(role == place_role and (path == place_path or path.startswith(place_path + "/")) for place_role, place_path in places)


def is_exempt(role, path):
    return is_under(role, path, EXEMPT) or path.rsplit("/", 1)[-1] == SESSION_KEY_NAME


def compare_to_the_state_read(before, after):
    """Every difference between two readings of the files off the write paths, each path with one expected value."""
    differences = []
    for key in sorted(set(before) | set(after)):
        role, path = key.split("\t", 1)
        if is_exempt(role, path):
            continue
        if key not in after:
            differences.append(f"missing {role}/{path}: {before[key]}")
        elif key not in before:
            if f"written-by={RELAY_PROBE_WRITER}" not in after[key].split():
                differences.append(f"extra {role}/{path}: {after[key]}")
        elif before[key] != after[key]:
            differences.append(f"differs {role}/{path}: {before[key]} -> {after[key]}")
    return differences


GUEST_DIRECTORY = "/usr/local/lib/internkim-rig"
FILE_JOURNAL_SOCKET_PATH = "/run/internkim-file-follow/barrier.sock"
WRITER_KINDS = ("database", "files", "messages")
LONGEST_WAIT_FOR_READY_SECONDS = 600
LONGEST_WAIT_FOR_WRITERS_SECONDS = 180
AUTOMATIC_TAKE_OVER_SECONDS = 300
LONGEST_TAKE_OVER_SECONDS = 1800
SCHEDULED_SECONDS_AFTER_THE_STATE = 120
SCHEDULED_INSTRUCTION = "Tell me in one short message that the reminder the native install rig asked for has come due."
LONGEST_WAIT_FOR_THE_SCHEDULE_SECONDS = 600
WRITING_BEFORE_THE_MOVE_SECONDS = 30
LONGEST_MOVE_INTERRUPTION_SECONDS = 60
LONGEST_MESSAGE_SECONDS = 180
MESSAGE_INTERVAL_SECONDS = 2
LONGEST_WAIT_FOR_THE_AGENT_AT_REST_SECONDS = 1200
LONGEST_HOLD_OF_THE_AGENT_SECONDS = 900
LONGEST_WAIT_FOR_THE_COMPANY_KEY_SECONDS = 600
FENCE_PROBE_SECONDS = 30
STORE_LINK_RATE = "100mbit"
FILL_MEBIBYTES = 1024
FILL_FILE_MEBIBYTES = 64
SETTLING_AFTER_THE_FILL_SECONDS = 120


@dataclass(frozen=True)
class MessageAttempt:
    writer: str
    index: int
    started: float
    finished: float
    identifier: str
    reason: str


class Acknowledgements:
    """What the writers acknowledge, kept on this Mac, where a guest losing its power cannot take it along."""

    def __init__(self, log_path, address):
        self.log_path = Path(log_path)
        self.received = []
        self.attempts = []
        self.lock = threading.Lock()
        self.message_sender = None
        self.message_counts = {}
        self.is_stopped = threading.Event()
        self.message_thread = None
        self.messages = threading.Condition()
        self.is_paused = False
        self.is_sending = False
        acknowledgements = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                kind, writer, value = (self.path.strip("/").split("/") + ["", "", ""])[:3]
                if kind not in WRITER_KINDS or not value.isdigit():
                    self.send_response(404)
                    self.end_headers()
                    return
                acknowledgements.record(kind, writer, int(value))
                self.send_response(200)
                self.end_headers()

            def log_message(self, format, *arguments):
                pass

        self.server = http.server.ThreadingHTTPServer((address, 0), Handler)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    @property
    def url(self):
        host, port = self.server.server_address[:2]
        return f"http://{host}:{port}"

    def record(self, kind, writer, value):
        with self.lock:
            received = time.time()
            self.received.append((received, kind, writer, value))
            with self.log_path.open("a") as log:
                log.write(f"{received:.3f}\t{kind}\t{writer}\t{value}\n")

    def of(self, kind, writer):
        with self.lock:
            return [value for _, held_kind, held_writer, value in self.received if held_kind == kind and held_writer == writer]

    def times_of(self, kind, writer, since):
        with self.lock:
            return [at for at, held_kind, held_writer, _ in self.received if held_kind == kind and held_writer == writer and at >= since]

    def attempts_of(self, writer, since):
        with self.lock:
            return [attempt for attempt in self.attempts if attempt.writer == writer and attempt.finished >= since]

    def keep_sending_messages(self, send, writer):
        """Send a counted message every few seconds for this writer until paused; send(index) answers (identifier, reason)."""
        with self.messages:
            self.message_sender = (send, writer)
            self.is_paused = False
            self.messages.notify_all()
        if self.message_thread:
            return
        self.message_thread = threading.Thread(target=self.send_messages, daemon=True)
        self.message_thread.start()

    def pause_the_messages(self):
        """Stop sending, and return once the message in flight, if any, has its answer."""
        with self.messages:
            self.is_paused = True
            while self.is_sending:
                self.messages.wait()

    def send_messages(self):
        while not self.is_stopped.is_set():
            with self.messages:
                while self.is_paused and not self.is_stopped.is_set():
                    self.messages.wait(1)
                if self.is_stopped.is_set():
                    return
                self.is_sending = True
                send, writer = self.message_sender
            try:
                self.send_one(send, writer)
            finally:
                with self.messages:
                    self.is_sending = False
                    self.messages.notify_all()
            self.is_stopped.wait(MESSAGE_INTERVAL_SECONDS)

    def send_one(self, send, writer):
        index = self.message_counts.get(writer, 0) + 1
        started = time.time()
        identifier, reason = send(index)
        finished = time.time()
        with self.lock:
            self.attempts.append(MessageAttempt(writer, index, started, finished, identifier, reason))
            with self.log_path.open("a") as log:
                log.write(f"{finished:.3f}\tattempt\t{writer}\t{index}\t{finished - started:.3f}s\t{identifier or reason}\n")
        if identifier:
            self.message_counts[writer] = index
            self.record("messages", writer, identifier)

    def wait_for_messages_after(self, writer, since, count, timeout_seconds):
        deadline = time.monotonic() + timeout_seconds
        while len(self.times_of("messages", writer, since)) < count and time.monotonic() < deadline:
            time.sleep(2)

    def stop_the_messages(self):
        self.is_stopped.set()
        with self.messages:
            self.messages.notify_all()
        if self.message_thread:
            self.message_thread.join(timeout=LONGEST_MESSAGE_SECONDS)
        self.server.shutdown()


def move_interruption(landed_at, pressed, served, writing_seconds):
    """The longest a member waited between two messages landing across a move, or None when it cannot say.

    It runs from the last message that landed before the administrator made another box serve to the first
    that landed once that box held the gateway. When no message landed in the writing before the move, the
    member's messages were not landing anyway, and a wait measured from then measures that, not the move.
    """
    landed = sorted(landed_at)
    before = [at for at in landed if pressed - writing_seconds <= at < pressed]
    after = [at for at in landed if served is not None and at >= served]
    if not before or not after:
        return None
    around = [at for at in landed if before[-1] <= at <= after[0]]
    return max(later - earlier for earlier, later in zip(around, around[1:]))


def message_round_trip(attempts):
    """The median seconds a member's message took to land, of the attempts that landed."""
    landed = [attempt.finished - attempt.started for attempt in attempts if attempt.identifier]
    return statistics.median(landed) if landed else None


def unanswered_reasons(attempts, since, until):
    """Each distinct reason a message did not land between two moments, with how often it was given."""
    counted = {}
    for attempt in attempts:
        if not attempt.identifier and since <= attempt.finished <= until:
            counted[attempt.reason] = counted.get(attempt.reason, 0) + 1
    return [f"{count}x {reason}" for reason, count in sorted(counted.items(), key=lambda item: -item[1])]


DATABASE_WRITER_SCRIPT = r"""
set -u
agent_url="$(sed -n 's/^DATABASE_URL=//p' %(environment)s | head -1)"
until psql "$agent_url" -qc "create table if not exists rig_written (writer text not null, n bigint not null, primary key (writer, n))" >/dev/null 2>&1; do sleep 1; done
n=1
while true; do
  if psql "$agent_url" -qtAc "insert into rig_written values ('%(writer)s', $n)" >/dev/null 2>&1; then
    until curl -fsS -o /dev/null --max-time 5 "%(acknowledgements)s/database/%(writer)s/$n"; do sleep 0.2; done
    n=$((n + 1))
  else
    sleep 0.5
  fi
done
"""

AGENT_DATABASE_COMMAND = """psql "$(sed -n 's/^DATABASE_URL=//p' %(environment)s | head -1)" --quiet --tuples-only --no-align --set ON_ERROR_STOP=1 --command %(statements)s 2>&1"""

WRITER_JOURNAL_COMMAND = "journalctl -u rig-database-writer-%(writer)s -u rig-file-writer-%(writer)s --no-pager -n 6 2>&1"

WRITTEN_ROWS_QUERY = "select n from rig_written where writer = '%(writer)s' order by n;"

FENCE_PROBE_COMMAND = r"""
url=%(url)s
[ -n "$url" ] || { echo "probe=the rig remembered no database URL for this box"; exit 0; }
timeout %(seconds)s psql "$url" --quiet --set ON_ERROR_STOP=1 --command "insert into rig_written values ('%(writer)s', 1)" 2>&1
printf 'insert=%%s\n' "$?"
printf 'kept=%%s\n' "$(timeout %(seconds)s psql "$url" -tAc "select count(*) from rig_written where writer = '%(writer)s'" 2>&1 | tr -d ' \n')"
"""


def fence_refused_the_commit(reading):
    """Whether the old box's database answered the probe's insert with a refusal and holds no probe row.

    An insert that waits past the probe's time is not refused: the row may be committed on the box while its
    commit waits, so it is judged by whether the row is there.
    """
    reported = dict(line.split("=", 1) for line in reading.splitlines() if line.startswith(("insert=", "kept=", "probe=")))
    if "probe" in reported:
        return False
    return reported.get("insert") not in ("0", "124") and reported.get("kept") != "1"


SCHEDULE_TOOL_PATH = "/memory/api/schedules/tool-create"
REQUESTER_EMAIL_HEADER = "X-INTERNKIM-REQUESTER-EMAIL"
LAUNCH_ORIGIN_EVENT = "blueclaw.launch.origin"
SCHEDULE_SESSION_PREFIX = "schedule:"

LAUNCH_ORIGIN_QUERY = (
    "select launched.body from task_event launched join task_run run using (task_run_id) "
    f"where launched.name = '{LAUNCH_ORIGIN_EVENT}' and run.requester_person_id = '%(person)s' "
    f"and coalesce(run.origin_conversation_id, '') not like '{SCHEDULE_SESSION_PREFIX}%%' "
    "order by launched.created_at desc limit 1"
)

SCHEDULE_RUN_QUERY = (
    "select 'runs=' || count(*) || ' first=' || coalesce(min(created_at)::text, 'none') from task_run "
    f"where origin_conversation_id = '{SCHEDULE_SESSION_PREFIX}%(schedule)s';"
    "select 'next_run_at=' || coalesce(next_run_at::text, 'none') || ' failures=' || failure_count || ' error=' || last_error "
    "from schedule where schedule_id = '%(schedule)s';"
)


def schedule_ran(reading):
    runs = next((word.split("=", 1)[1] for word in reading.split() if word.startswith("runs=")), "0")
    return runs.isdigit() and int(runs) > 0


class AdmindConnection(http.client.HTTPConnection):
    def __init__(self, socket_path):
        super().__init__("internkim", timeout=60)
        self.socket_path = socket_path

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX)
        self.sock.settimeout(self.timeout)
        self.sock.connect(self.socket_path)


def asked_of_admind(socket_path, path, requester_email, body):
    connection = AdmindConnection(socket_path)
    try:
        connection.request("POST", path, json.dumps(body), {"Content-Type": "application/json", REQUESTER_EMAIL_HEADER: requester_email})
        response = connection.getresponse()
        return response.status, response.read().decode(errors="replace")
    finally:
        connection.close()


def schedule_a_task(environment_path, socket_path, requester_email, person_id, seconds, instruction):
    """What capabilityd asks admind when the agent calls schedule_create in a person's direct conversation.

    The delivery binding is the one the agent recorded when it launched that person's newest task there, which is
    the context capabilityd would add to the call. The schedule is not approved: only the approval gate approves.
    """
    origin = agent_database_answer(environment_path, LAUNCH_ORIGIN_QUERY % {"person": person_id})
    binding = json.loads(origin) if origin.startswith("{") else {}
    run_at = datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=int(seconds))
    body = {
        "taskInstruction": instruction,
        "description": "rig schedule",
        "kind": "once",
        "runAt": run_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "platform": binding.get("platform", ""),
        "conversationID": binding.get("conversationID", ""),
        "replyTargetID": binding.get("replyTargetID", ""),
        "isApproved": False,
    }
    status, answer = asked_of_admind(socket_path, SCHEDULE_TOOL_PATH, requester_email, body)
    created = json.loads(answer) if status == 200 and answer.startswith("{") else {}
    return {"status": status, "scheduleID": created.get("scheduleID", ""), "runAt": body["runAt"], "binding": origin, "answer": answer[:600]}


BOX_ASSERTION_AUDIENCE = "internkim-box"
BOX_ASSERTION_LIFETIME_SECONDS = 60
ED25519_PRIVATE_KEY_PREFIX = bytes.fromhex("302e020100300506032b657004220420")


def unpadded(data):
    return base64.urlsafe_b64encode(data).decode().rstrip("=")


def box_assertion(identity_path):
    """A box session assertion signed with the box identity key in identity_path, as internal/box/assertion.go signs one."""
    with open(identity_path) as identity:
        seed = json.load(identity)["signingSeed"]
    with tempfile.TemporaryDirectory() as directory:
        key_path = os.path.join(directory, "key.der")
        with open(key_path, "wb") as key:
            key.write(ED25519_PRIVATE_KEY_PREFIX + base64.urlsafe_b64decode(seed + "=" * (-len(seed) % 4)))
        public_key = subprocess.run(
            ["openssl", "pkey", "-inform", "DER", "-in", key_path, "-pubout", "-outform", "DER"], capture_output=True, check=True
        ).stdout[-32:]
        issued = int(time.time())
        header = unpadded(json.dumps({"alg": "EdDSA", "typ": "JWT"}).encode())
        claims = unpadded(json.dumps({"iss": unpadded(public_key), "aud": BOX_ASSERTION_AUDIENCE, "iat": issued, "exp": issued + BOX_ASSERTION_LIFETIME_SECONDS}).encode())
        signing_input_path = os.path.join(directory, "signing-input")
        with open(signing_input_path, "w") as signing_input:
            signing_input.write(header + "." + claims)
        signature = subprocess.run(
            ["openssl", "pkeyutl", "-sign", "-inkey", key_path, "-keyform", "DER", "-rawin", "-in", signing_input_path],
            capture_output=True,
            check=True,
        ).stdout
    return header + "." + claims + "." + unpadded(signature)


AGENT_AT_REST_COMMAND = r"""
agent_url="$(sed -n 's/^DATABASE_URL=//p' %(environment)s 2>/dev/null | head -1)"
printf 'running=%%s\n' "$(psql "$agent_url" -tAc "select coalesce(string_agg(status, ','), '') from task_run where status in ('planned', 'running')" 2>&1 | tr -d ' \n')"
printf 'taken=%%s\n' "$(psql "$agent_url" -tAc "select count(*) from task_run where position('%(token)s' in prompt) > 0" 2>&1 | tr -d ' \n')"
printf 'delivered=%%s\n' "$(grep -c '%(message)s' %(delivered)s 2>/dev/null || true)"
"""


def agent_is_at_rest(reading):
    """No turn of the agent running, its newest message taken in, and the relay holding nothing inbound."""
    reported = dict(line.split("=", 1) for line in reading.splitlines() if "=" in line)
    is_taken_in = reported.get("taken", "0") not in ("", "0") or reported.get("delivered", "0") not in ("", "0")
    return reported.get("running") == "" and reported.get("queued") == "0" and is_taken_in


STORED_MESSAGES_COMMAND = r"""
url="$(sed -n 's/^DATABASE_URL=//p' %(path)s 2>/dev/null | head -1)"
psql "$url" -tAc "select encode(id, 'hex') from events where kind = 9 and encode(id, 'hex') in (%(identifiers)s)"
"""

STORE_LINK_LIMIT_COMMAND = r"""
set -e
command -v tc >/dev/null || { export DEBIAN_FRONTEND=noninteractive; apt-get install -y iproute2 >/dev/null; }
device="$(ip route show default | awk '{ print $5; exit }')"
tc qdisc del dev "$device" root 2>/dev/null || true
tc qdisc del dev "$device" ingress 2>/dev/null || true
tc qdisc add dev "$device" root handle 1: htb default 20
tc class add dev "$device" parent 1: classid 1:10 htb rate %(rate)s ceil %(rate)s
tc class add dev "$device" parent 1: classid 1:20 htb rate 10gbit
tc filter add dev "$device" parent 1: protocol ip prio 1 u32 match ip dport %(port)s 0xffff flowid 1:10
tc qdisc add dev "$device" handle ffff: ingress
tc filter add dev "$device" parent ffff: protocol ip prio 1 u32 match ip sport %(port)s 0xffff police rate %(rate)s burst 1m drop flowid :1
tc class show dev "$device"
tc filter show dev "$device" parent ffff:
"""

FILL_COMMAND = r"""
set -eu
person="$(ls -1 %(people)s | head -1)"
owner="$(stat -c %%U %(people)s/$person)"
directory="%(people)s/$person/%(prefix)sfill"
runuser -u "$owner" -- mkdir -p "$directory"
started="$(date +%%s)"
for index in $(seq 1 $(( %(mebibytes)s / %(file_mebibytes)s ))); do
  runuser -u "$owner" -- dd if=/dev/urandom of="$directory/$index.bin" bs=1M count=%(file_mebibytes)s status=none
done
answer="$(runuser -u "$owner" -- %(python)s %(module)s ask-the-barrier %(socket)s)"
printf 'wrote %%s MiB in %%ss; the barrier answered %%s\n' %(mebibytes)s "$(( $(date +%%s) - started ))" "$answer"
"""

BARRIER_REQUEST = "wait"
BARRIER_KEPT = "kept"
BARRIER_WAIT_SECONDS = 40


def barrier_answer(socket_path):
    with socket.socket(socket.AF_UNIX) as connection:
        connection.settimeout(BARRIER_WAIT_SECONDS)
        connection.connect(socket_path)
        connection.sendall((BARRIER_REQUEST + "\n").encode())
        return connection.makefile().readline().strip()


def write_files(directory, acknowledgements, writer, socket_path):
    """A person writing file after file, each acknowledged only once the file barrier says it is kept."""
    os.makedirs(directory, exist_ok=True)
    index = 1
    while True:
        try:
            with open(os.path.join(directory, f"{index}.txt"), "w") as target:
                target.write(file_content(index))
            answer = barrier_answer(socket_path)
        except OSError as failure:
            print(f"write {index}: {failure}", flush=True)
            time.sleep(0.5)
            continue
        if answer != BARRIER_KEPT:
            print(f"write {index}: the barrier answered {answer!r}", flush=True)
            time.sleep(0.5)
            continue
        while True:
            try:
                urllib.request.urlopen(f"{acknowledgements}/files/{writer}/{index}", timeout=5).read()
                break
            except OSError:
                time.sleep(0.2)
        index += 1


def files_written(directory):
    found = {}
    for name in os.listdir(directory) if os.path.isdir(directory) else []:
        if name.endswith(".txt") and name[:-4].isdigit():
            with open(os.path.join(directory, name)) as source:
                found[int(name[:-4])] = source.read()
    return found


def digest_of(path):
    """A file's SHA-256, a SQLite database's by the SQL its dump prints, a connection file's without its session."""
    if is_a_database(path):
        connection = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
        try:
            return "dump:" + hashlib.sha256("\n".join(connection.iterdump()).encode()).hexdigest()
        finally:
            connection.close()
    if path.rsplit("/", 1)[-1] == CONNECTION_FILE_NAME:
        with open(path) as source:
            document = json.load(source)
        document.pop(CONNECTION_SESSION_FIELD, None)
        return "without-session:" + hashlib.sha256(json.dumps(document, sort_keys=True).encode()).hexdigest()
    hashed = hashlib.sha256()
    with open(path, "rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            hashed.update(block)
    return hashed.hexdigest()


SQLITE_HEADER = b"SQLite format 3\x00"
SQLITE_SIDE_SUFFIXES = ("-wal", "-shm", "-journal")
CONNECTION_FILE_NAME = "connection.json"
CONNECTION_SESSION_FIELD = "agentKey"


def is_a_relay_probe_object(path, size):
    if size > RELAY_PROBE_LARGEST_BYTES:
        return False
    with open(path, "rb") as source:
        return RELAY_PROBE_BODY.fullmatch(source.read()) is not None


def is_a_database(path):
    if not os.path.isfile(path) or os.path.islink(path):
        return False
    with open(path, "rb") as source:
        return source.read(len(SQLITE_HEADER)) == SQLITE_HEADER


def is_beside_a_database(path):
    """A -wal, -shm or -journal file, whose content the dump of the database beside it reads through."""
    return any(path.endswith(suffix) and is_a_database(path[: -len(suffix)]) for suffix in SQLITE_SIDE_SUFFIXES)


def entry_of(path):
    def name_of(number, lookup):
        try:
            return lookup(number)[0]
        except KeyError:
            return str(number)

    held = os.lstat(path)
    kind = {stat.S_IFLNK: "l", stat.S_IFDIR: "d", stat.S_IFREG: "f"}.get(stat.S_IFMT(held.st_mode), "o")
    described = {
        "type": kind,
        "mode": oct(stat.S_IMODE(held.st_mode)),
        "owner": name_of(held.st_uid, pwd.getpwuid),
        "group": name_of(held.st_gid, grp.getgrgid),
    }
    if kind == "l":
        described["target"] = os.readlink(path)
    if kind == "f":
        described["digest"] = digest_of(path)
        if is_a_relay_probe_object(path, held.st_size):
            described["written-by"] = RELAY_PROBE_WRITER
    return " ".join(f"{field}={value}" for field, value in described.items())


def files_off_the_write_paths(roots):
    entries = {}
    for role, root in roots:
        for directory, directories, names in os.walk(root):
            directories[:] = [name for name in directories if not name.startswith(WRITE_PATH_PREFIX)]
            for name in directories + names:
                path = os.path.join(directory, name)
                if not is_beside_a_database(path):
                    entries[role + "\t" + os.path.relpath(path, root)] = entry_of(path)
    return entries


def projected_accounts():
    people = sorted(
        f"{entry.pw_name}:{entry.pw_uid}:{entry.pw_gid}:{entry.pw_dir}:{entry.pw_shell}"
        for entry in pwd.getpwall()
        if entry.pw_name.startswith("bc_")
    )
    groups = sorted(
        f"{entry.gr_name}:{entry.gr_gid}:{','.join(sorted(entry.gr_mem))}" for entry in grp.getgrall() if entry.gr_name.startswith("bc_")
    )
    return people + groups


SCHEDULES_QUERY = (
    "select coalesce(json_agg(row_to_json(kept) order by kept.schedule_id), '[]') from (select schedule_id, name, "
    "prompt, execution_mode, schedule_kind, run_at, interval_second, cron_expression, is_paused, time_zone, "
    "expires_at, max_run_count from schedule) kept"
)


def agent_database_answer(environment_path, query):
    url = ""
    with open(environment_path) as environment:
        for line in environment:
            if line.startswith("DATABASE_URL="):
                url = line.strip().split("=", 1)[1]
    answered = subprocess.run(["psql", url, "-tAc", query], capture_output=True, text=True)
    return answered.stdout.strip() if answered.returncode == 0 else "refused: " + answered.stderr.strip()[-300:]


def roster_digest(agent_unit):
    started = subprocess.run(["systemctl", "show", agent_unit, "-p", "ExecStart", "--value"], capture_output=True, text=True).stdout
    words = started.replace(";", " ").split()
    path = words[words.index("-policy") + 1] if "-policy" in words else ""
    if not path or not os.path.exists(path):
        return f"no roster: {agent_unit} names no -policy file that exists"
    return digest_of(path)


def secret_digests(secrets_directory):
    names = sorted(os.listdir(secrets_directory)) if os.path.isdir(secrets_directory) else []
    return {name: digest_of(os.path.join(secrets_directory, name)) for name in names if name == "openrouter-key" or "mail" in name}


def read_the_state(roots, environment_path, agent_unit, secrets_directory):
    """Everything off the write paths a take-over has to carry, read as root on the box that serves."""
    return {
        "files": files_off_the_write_paths(roots),
        "accounts": projected_accounts(),
        "schedules": agent_database_answer(environment_path, SCHEDULES_QUERY),
        "roster": roster_digest(agent_unit),
        "secrets": secret_digests(secrets_directory),
    }


def main(arguments):
    command, *rest = arguments
    if command == "write-files":
        write_files(*rest)
    elif command == "ask-the-barrier":
        print(barrier_answer(*rest))
    elif command == "files-written":
        print(json.dumps(files_written(*rest)))
    elif command == "read-the-state":
        roots, environment_path, agent_unit, secrets_directory = rest
        print(json.dumps(read_the_state(json.loads(roots), environment_path, agent_unit, secrets_directory)))
    elif command == "schedule-a-task":
        print(json.dumps(schedule_a_task(*rest)))
    elif command == "box-assertion":
        print(box_assertion(*rest))
    else:
        raise SystemExit(
            f"no such command {command!r}; the commands are write-files, ask-the-barrier, files-written, read-the-state, "
            "schedule-a-task and box-assertion"
        )


if __name__ == "__main__":
    main(sys.argv[1:])
