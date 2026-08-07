---
title: What lives where
order: 40
---

One question decides it. If that computer burned down tonight, what has to
still be true tomorrow?

Whatever answers yes belongs in the record. Whatever is a fact about the
machine belongs on the host.

## The record

Ten tables, and each is something a company would still need after replacing
every computer it owns.

| | |
|---|---|
| `company` · `member` · `team` | who this is and who works here |
| `credential` · `messenger_person` | which messenger identity is which person |
| `agent` | the key a host proves itself with |
| `attendance` · `leave` | when people worked and when they were away |
| `task` · `task_participant` | work and calendar events, with participants |

Calendar events are tasks with a span, which is why there is no separate table
for them.

## The agent's store

Twenty-nine tables on the host, in four groups.

| | |
|---|---|
| Conversation | raw events, conversations, attachments, message segments |
| Execution | task runs, attempts, steps, the event ledger, schedules |
| Memory | memory records and their sources, the episode graph |
| Policy | circles, channel rules, access rules, revisions |

None of it is a company record. It is the agent's working state: what was
said, what it did, what it remembers, and what it is allowed to touch.

## Files

The workspace lives on the host, under the permissions described in
[Boundaries](/docs/boundaries). Private space per person, shared space per
group, and a service-owned area the agent's own machinery uses.

Backing that up is the company's job. Losing the host loses the agent's memory
and the files, and loses none of the company's records.

## Where the line is still moving

Two things sit on the host today that arguably answer yes to the question
above.

**The agent's memory.** It would survive a machine if it were central, and it
is bound to execution where it is. Nothing has been decided.

**Customer relationship records.** They belong with the other company records,
and the record has no table for them yet.
