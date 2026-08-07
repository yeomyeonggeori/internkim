---
title: Boundaries
order: 30
---

Two things decide what may happen, and each is complete in its own territory.

| | Guards | Keyed on | Enforced by |
|---|---|---|---|
| Row level security | company records | `member` | the database |
| POSIX permissions | files and execution | a user per person | the operating system |

Neither imitates the other, and no third layer sits above them deciding access
in application code.

## Records

Every read and write against the record happens as a member. The app does it
with the session of whoever signed in; the agent does it with a session the
control plane issued for the person it is acting for. The one key that could
bypass this lives on the server side of the control plane, and what it issues
is a session.

So **the agent has no privileged view.** Acting for someone, it sees what that
person sees. Acting for an administrator, it sees what an administrator sees.

## Files and execution

On the host, each person is a real operating-system user, each group of people
a real group. The agent runs a person's work as that user, so the workspace,
the terminal, and any program it starts are bounded by ordinary file
permissions.

There is no list of allowed commands. A command that would modify the system
fails because the user running it may not, which is the same reason it would
fail for that person at a shell.

## The one place they meet

A person is a `member` in the record and a user on the host. One mapping joins
those two facts, and it is the only bridge between the two boundaries. Identity
resolution is exact: an identifier matches or it does not.

## What this rules out

- Deciding access by matching path strings. Ownership and mode bits decide.
- Deciding meaning by matching words. Structured answers and exact identifier
  resolution do that instead.
- A second permission model in application code that the database and the
  operating system know nothing about.
