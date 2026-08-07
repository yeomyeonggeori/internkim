---
title: What InternKim is
order: 10
---

InternKim is a coworker your company runs itself. It does what an assistant
does: records attendance, keeps the calendar, drafts and files things, runs what
needs running.

It is split across two machines, and which one holds a thing is decided by a
single question: **if that computer burned down tonight, what has to still be
true tomorrow?**

## Two ways to reach it

**The web app is the default, and everything is in it.** Attendance, leave, the
calendar, work, the org chart, settings, and a messenger screen where people
talk to InternKim and to each other. Someone who only ever opens the web app is
missing nothing.

**A messenger app works too**, wherever that messenger is reachable from. The
chat is your company's own Slack, Mattermost, or Buzz relay, so its ordinary
apps sign in and work as usual. A messenger kept inside the office is reachable
inside the office; the web app's chat screen is how the same conversation
follows someone onto a train. See [where the messenger server
runs](/docs/architecture).

Both reach one conversation. The web app's messenger screen is a view onto the
company's messenger rather than a separate inbox, so a message written in
either place arrives in the other, and the agent hears it the same way
regardless of which one sent it.

## The records

A Supabase project. Postgres holds the company's records, row level security
decides who may read a row, Supabase Auth signs people in, and Realtime is how
a browser reaches a computer it cannot connect to directly.

Alongside it, a static web app that people sign into, and a handful of server
routes holding the one key that can bypass row level security. Those routes
issue what a member cannot issue alone: a session for an agent, a new company,
an invitation.

This documentation names those three separately, because they behave
differently even though they deploy together. See the
[glossary](/docs/glossary).

The project can be a hosted one or your own. The schema is a directory of
migrations, the app is a static build, and which project the host talks to is
configuration. Self-hosted Supabase ships the same Postgres, the same row level
security, and the vault extension the schema uses. Dumping a project and
restoring it into an empty database built only from the migrations brings the
accounts with it, so people sign in afterwards with the passwords they had.

## The host

One computer that stays on. A Linux server, a Mac that does not sleep, a box
in a cupboard. It runs the agent, the model it talks to, the connector to your
messenger, and a Postgres of its own holding conversations, what the agent did,
and what it remembers.

Nothing on it listens. Every connection it makes goes outward, so there is no
tunnel to keep alive, no port to forward, and no hostname pointing at a machine
in your office.

## What stays yours

**The messenger.** InternKim attaches to the Slack, Mattermost, or Buzz relay
you already run. No conversation reaches the records, because the schema has no
table for one.

**The model.** The agent talks to whichever provider you configure, or to a
model running on your own box.

**The computer.** Replaceable at any time. Everything a company would still
need after replacing every machine it owns lives in the records, not on the
box.

## Where to go next

| | |
|---|---|
| [Architecture](/docs/architecture) | what talks to what, in one diagram |
| [Boundaries](/docs/boundaries) | the two things that decide who may do what |
| [What lives where](/docs/what-lives-where) | which data is ours to hold and which is yours |
| [Running the host](/docs/running-the-host) | what the always-on computer needs |
| [Glossary](/docs/glossary) | the words this documentation uses precisely |
