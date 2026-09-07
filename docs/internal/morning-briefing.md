# Morning briefing

Every person in the company policy roster has a managed daily briefing. The
runtime derives its time from the person's `.internkim/user.json` and the
company time zone. Existing profiles default to:

```json
{
  "morningBriefing": {
    "enabled": true,
    "time": "08:00"
  }
}
```

The user can ask the agent to change the time or disable the briefing through
`persona_update`. The profile remains the configuration owner. Generic schedule
update, cancel and delete operations cannot change a managed briefing.

The trigger is the next daily occurrence in `company.timeZone`, including
weekends. New or reenabled settings take effect at the next future occurrence.
The runtime resolves a linked messenger account to a real private conversation
with the agent. If several platforms are linked, the platform identifier's
lexical order selects one; conflicting accounts on that platform stop delivery.
A missing account leaves the briefing inactive until an account is linked.

The briefing queries the requester's current work and calendar. It covers
today's events and deadlines, ongoing and overdue work, and upcoming planned
work. Memory can supply context for these records. The task has only read tools;
its final response is delivered through the existing transactional outbox.
Configuration and the recipient are checked again before that response is
enqueued. Disabling a briefing does not retract a message already queued.

Execution state survives runtime restarts. Claimed leases and occurrence keys
guard delivery. After the attempt limit, the failed occurrence keeps its error
and the briefing moves to the next day without counting the failure as a
completed briefing. Invalid profiles stop that person's briefing and produce a
diagnostic. An unavailable company time zone stops briefings until it recovers.

## Acceptance

Run `./internkim dev simulate --scenario schedule_lifecycle_acceptance` for the
existing scheduler baseline. Run the persona, scheduler, agentruntime and
Postgres unit suites for defaults, scope, time zones, profile changes and
managed-state invariants. The Postgres lifecycle test requires
`BLUECLAW_TEST_POSTGRES_URL` in a disposable Local Fleet and creates its own
temporary database.

`./internkim dev fleet run --scenario morning-briefing` checks the actual Linux
profile boundary, protected schedule, near-future occurrence, task/calendar
queries and one private delivery. Evidence stays in the fleet run artifacts.
The costed `TestMorningBriefingSettingsLive` test exercises the real
model's profile-update decision separately.

For manual memory acceptance, use a dedicated test identity. Store a unique
work fact, inspect its source in the memory screen, and ask about it in a new
conversation using different wording. Inspect the task ledger for a memory
lookup and its returned fact. Then correct and delete the fact, repeating the
lookup after each change. History can still contain a deleted fact; the ledger
must distinguish history use from memory retrieval. An empty-memory control
must not invent the answer. Remove test records and messages afterward.
