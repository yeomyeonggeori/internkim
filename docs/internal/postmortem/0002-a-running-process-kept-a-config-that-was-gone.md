# 0002 — A running process kept a config that was gone

Forty minutes in which no message became a task, on a device that reported
healthy the whole time.

## What it looked like

Process up. `systemctl` green. Admin URL answering 200. Messages arriving and
nothing happening to them.

## What was actually wrong

The release removed capabilityd's llmd bridge and shipped the `admind` that
stops naming it in the config. Both were correct. Neither restarted `blueclaw`.

A running process holds the configuration it started with, so the already-running
`blueclaw` kept pointing at a bridge that no longer existed. Its protocol
identity check failed and it fell through to `serveWithoutStartingWork`: a mode
that answers HTTP and refuses all work.

Shipping `blueclawPayload` in the same release would have restarted it onto the
matching config.

## Why the usual recovery did not apply

Nothing in the service table distinguishes this from health. `systemctl
is-active` reports the process, not whether the process is doing its job, and a
component that answers its readiness endpoint while refusing work satisfies
every check a deploy runs.

The signal that does exist is task-shaped: `internkim task list` showing no run
newer than the deploy.

## What would have caught it earlier

Treating "who reads this config" as part of the change that removes what the
config names. Shipping the writer and the removal without the reader leaves the
reader pointed at something gone, and the failure surfaces as silence rather
than as an error.
