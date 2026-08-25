# 0003 — The agent roster fell back to its bootstrap account

A company of seven, an agent that knew nobody, and the first anyone heard of it
was an employee being told they had never been invited.

## What it looked like

An admin asked the agent in the messenger to record a meeting. The reply:

> This Intern Kim has not invited your account yet. Ask the administrator for
> access. Your account presents leesample@example.com, and no person here is registered
> with that address.

The person is an active admin with a claimed account. The device answered HTTP
on every endpoint, every service reported active, and the release that had just
been applied verified clean.

## What was actually wrong

The agent matches an inbound messenger account against the people its own policy
carries. That policy held one entry:

```
GET /admin/api/policy → {"people":[{"emails":["admin@example.test"], …}]}
```

`admin@example.test` is the bootstrap account a fresh device starts with. All
seven employees were still in the host's other roster, the fleet users index
`internkim users list` reads, which is a different store written by a different
path. Nothing compares the two.

So the refusal was true of what the agent could see and false about the company.
Whether someone was invited is decided in the account directory, which the agent
does not read at all.

## Why the usual recovery did not apply

Every check that runs after a deploy passed. `release status` showed each
component matching the tree with no drift marker. `systemctl` showed four green
services. The admin URL answered. The task ledger held runs, all of them from
before the roster emptied, and an idle agent looks exactly like a working one
until someone writes to it.

The one signal that existed was a person being refused, which arrives through a
human rather than through the system.

## What would have caught it earlier

An emptied projection is mechanically detectable. A host that has served real
people and now holds only the account it was born with has lost its roster, and
that is a fault it can report at boot and at every policy load, rather than a
state it can quietly serve refusals from.

The deeper cause is that the roster is a copy nobody owns. The rule that
replaces it is written in
[saas-design §5.1](../saas-design.md#51-the-hosts-roster-is-derived-and-an-empty-derivation-is-a-fault):
the host's roster is derived from the central account directory and never edited
on the host. Three hand-written copies of who works here will drift again
otherwise, and drift stays invisible until it refuses somebody.

One smaller thing also came out of it. The refusal asserted an invitation status
the agent has no way to read, and an administrator who believes it invites a
person who already has a record, which is how one person ends up with two.
[blueclaw#115](https://github.com/Dawn-kim-official/blueclaw/pull/115) makes the
message state only the match that failed.
