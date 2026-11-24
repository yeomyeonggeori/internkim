"""Step 16: a host whose clock boots at 1970 hands the agent no message it already let go.

A box without a battery-backed clock starts at the epoch and learns the time from the
network some seconds later. chatd asked the messenger for "messages since now" in that
window, got the whole history, and the agent answered months-old requests again
(yeomyeonggeori/internkim#3115). The rig leaves a message dated a month back in the
messenger, restarts chatd with the clock at 1970, and reads whether the agent ever took
that message up.
"""

import time

CHATD_UNIT_NAME = "chatd.service"
MONTH_SECONDS = 30 * 24 * 60 * 60
JANUARY_1970_SECONDS = 24 * 60 * 60
CLOCK_TOLERANCE_SECONDS = 60
CATCH_UP_WAIT_SECONDS = 180

STOPPED_CLOCK_COMMAND = "timedatectl set-ntp false 2>/dev/null; date -u -s @%(seconds)d >/dev/null; date -u +%%s"
FOLLOWED_CLOCK_COMMAND = "date -u -s @%(seconds)d >/dev/null; timedatectl set-ntp true; date -u +%%s"
RESTART_COMMAND = f"systemctl restart {CHATD_UNIT_NAME} && systemctl is-active {CHATD_UNIT_NAME}"

TAKEN_UP_COMMAND = r"""
agent_url="$(sed -n 's/^DATABASE_URL=//p' %(agent_path)s 2>/dev/null | head -1)"
messenger_url="$(sed -n 's/^DATABASE_URL=//p' %(messenger_path)s 2>/dev/null | head -1)"
printf 'task=%%s\n' "$(psql "$agent_url" -tAc "select status from task_run where position('%(text)s' in prompt) > 0 order by created_at desc limit 1" 2>&1 | tr -d ' \n')"
printf 'replies=%%s\n' "$(psql "$messenger_url" -tAc "select count(*) from events reply join events asked on encode(asked.id,'hex') = '%(message)s' where reply.kind = 9 and reply.channel_id is not distinct from asked.channel_id and reply.pubkey <> asked.pubkey" 2>&1 | tr -d ' \n')"
"""


def set_the_clock(machine, seconds):
    """Stops time synchronisation and sets the guest's clock, and reports the clock it reads back.

    The kernel refuses a wall clock earlier than the time since boot, so a clock that has just
    booted reads a day into 1970 rather than its first second.
    """
    completed = machine.shell(STOPPED_CLOCK_COMMAND % {"seconds": seconds})
    return {"wanted": seconds, "read": completed.stdout.strip(), "refused": completed.stderr.strip()[-300:]}


def reads(clock):
    read = clock["read"]
    return read.isdigit() and abs(int(read) - clock["wanted"]) <= CLOCK_TOLERANCE_SECONDS


def give_the_clock_back(machine):
    """Sets the guest's clock to the rig's and lets the network keep it."""
    return machine.shell(FOLLOWED_CLOCK_COMMAND % {"seconds": int(time.time())}).stdout.strip()


def leave_a_message_from_a_month_ago(rig, text):
    """A member's message the messenger keeps dated a month back, sent while the guest's clock reads then.

    chatd signs a member's message with the guest's clock, so turning that clock back is the
    only way to give the messenger an old message signed the way every other one is.
    """
    month_ago = int(time.time()) - MONTH_SECONDS
    clock = set_the_clock(rig.machine, month_ago)
    session = rig.plane.member_session(rig.company_id, text)
    sent = session.get("sent") or {}
    return {
        "clock": clock,
        "messageID": sent.get("messageID") or "",
        "status": sent.get("status"),
        "refused": session.get("refused", ""),
    }


def restart_at_the_epoch(machine):
    """Restarts chatd with the clock at 1970, the moment a box without a clock battery boots into."""
    clock = set_the_clock(machine, JANUARY_1970_SECONDS)
    restarted = machine.shell(RESTART_COMMAND).stdout.strip()
    return {"clock": clock, "chatd": restarted}


def reading_of(text):
    return dict(line.split("=", 1) for line in text.splitlines() if "=" in line)


def was_taken_up(before, after):
    """Whether the agent opened a task for the message, or someone else wrote in its conversation since.

    Replies are counted before and after rather than filtered by time, because a reply written
    while the clock reads 1970 is dated before the message it answers.
    """
    replies_before, replies_after = before.get("replies", ""), after.get("replies", "")
    has_more_replies = replies_before.isdigit() and replies_after.isdigit() and int(replies_after) > int(replies_before)
    return bool(after.get("task")) or has_more_replies


def observe_a_restart_at_the_epoch(rig, step, text, agent_path, messenger_path):
    """Leaves a month-old message, restarts chatd at 1970, and judges whether the agent took it up."""

    def taken_up(message_id):
        command = TAKEN_UP_COMMAND % {
            "agent_path": agent_path,
            "messenger_path": messenger_path,
            "text": text,
            "message": message_id or "no-message",
        }
        return reading_of(rig.machine.shell(command).stdout)

    try:
        left = leave_a_message_from_a_month_ago(rig, text)
        before = taken_up(left["messageID"])
        restarted = restart_at_the_epoch(rig.machine)
        time.sleep(CATCH_UP_WAIT_SECONDS)
        after = taken_up(left["messageID"])
    finally:
        given_back = give_the_clock_back(rig.machine)

    step.observe(
        "a message dated a month back lands in the messenger",
        "the member's own send of it, answered by the company computer while the guest's clock read a month ago",
        "without it in the messenger, nothing below tests a replay",
    ).record(reads(left["clock"]) and bool(left["messageID"]), str(left))
    step.observe(
        "chatd restarted with the clock at 1970 hands the agent no message older than what it already handed over",
        f"the agent's task ledger and the messenger's store for the month-old message, read {CATCH_UP_WAIT_SECONDS} "
        "seconds after chatd restarted at the epoch, longer than one catch-up",
        "chatd being active after the restart says it came up; the replay #3115 found came from a chatd that came up fine",
    ).record(
        reads(left["clock"]) and reads(restarted["clock"]) and bool(left["messageID"]) and not was_taken_up(before, after),
        f"restart: {restarted}\nbefore the restart: {before}\nafter: {after}\nclock given back: {given_back}",
    )
