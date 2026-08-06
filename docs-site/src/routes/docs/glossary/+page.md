---
title: Glossary
order: 60
---

These words are used precisely throughout this documentation, and several of
them name things that are easy to confuse.

## Supabase

The platform the three central pieces are built on: Postgres with row level
security, an authentication service, a Realtime channel service, and an HTTP
API over the tables.

Naming the platform answers "where is it hosted". It does not answer "what
wrote this row", which is why the three names below exist.

## There is no word for "all of it"

An earlier version of this documentation called the whole central side "the
central plane". That name is retired, because it covered three things at once
and left every sentence using it ambiguous: a database that answers as a
member, a set of routes that answer as an administrator, and a page that holds
no key at all.

Say which one you mean.

## The record

The Postgres database holding company data. Row level security is its access
boundary, so every read and write happens as some member.

Say "the record" rather than "the database": the host has a database too, and
it holds something else entirely.

## The control plane

The small set of server routes holding the key that bypasses row level
security. It issues things a member cannot issue for themselves, and it hands
back sessions rather than data.

Not the record, and not the app, though all three are deployed together.

## The app

The static page a person signs into, and the default client. It holds no
secret, reads the record as whoever is signed in, and calls the host over a
Realtime channel for the one thing it cannot read directly: the chat screen.

## The chat screen

The part of the app that shows the company's messenger. It is a view onto that
messenger, so what appears in it appears in the messenger's own apps too.

## A messenger app

Slack's app, Mattermost's app, a Buzz client: the ordinary software a person
opens to read the chat. The second way to reach InternKim, from anywhere the
messenger server itself is reachable.

Distinct from that server, which is the company's own and may sit in a vendor's
cloud, on the company network, or on the host computer.

## The host

The company's always-on computer, and everything running on it. One company,
one host.

## The bridge

The process that answers the chat screen. It receives a call over a Realtime
channel and turns it into messenger operations.

**A bridge is not a proxy.** A proxy forwards a request to a server, preserving
it. The bridge translates: the paths it receives are its own, and it does work
the messenger has no notion of, such as building link previews and keeping the
list of who is who current. Elsewhere in this system "proxy" names things that
genuinely forward bytes, so the two words stay apart.

## The connector

The process that attaches to the company's messenger and turns arriving
messages into work for the agent. Separate from the bridge, and pointed the
other way: the bridge serves the browser, the connector serves the agent.

## The agent's store

The Postgres database on the host. Conversations, task runs, memory, and
policy. Never company records.

## Two words that mean two things

| Word | In the record | On the host |
|---|---|---|
| `task` | a piece of work or a calendar event | nothing; agent runs are `task_run` |
| `person` | nothing; people are `member` | a read-only projection of policy |

When either word could be ambiguous, this documentation says "a task in the
record" or "an agent run".
