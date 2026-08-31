# 0001 — A release check read a copy nothing writes

One working day. Every deploy failed, and blueclaw stopped on a device people
were using.

## What it looked like

A release that had worked the day before failed. `--components admind`, the
usual way to ship a fix to the thing that applies releases, failed the same way:
a release describes the whole device, so it inherited the component that could
not be accepted. The device was now unable to receive the release carrying the
fix for the reason it was failing.

## What was actually wrong

[#560](https://github.com/yeomyeonggeori/internkim/pull/560) moved the
blueclaw payload to the delivery share. `blueclawWorkspaceManifestMatchesTarget`
kept reading the workspace image, so the check was comparing the artifact
against a copy nothing writes any more.

That comparison passed for as long as the payload never changed. The first
release that changed it turned every subsequent release into a failure.

## Why the usual recovery did not apply

The release is applied by the `admind` already running. New code that changes
what an install verifies does not change what the old `admind` verifies, so a
device stuck this way cannot be fixed by sending it a better release — the old
process is still the one deciding.

The door out is the SSH path, which does not go through the release engine:

```
make build
./internkim setup --only admind --force
```

`--force` is required because the step is satisfied by "the file exists and the
service is active", which is true of a device running months-old code.
`systemctl show internkim-admind -p ExecMainStartTimestamp` says whether it
worked: a timestamp older than the deploy means the old process is still the one
applying releases.

## What would have caught it earlier

Grepping for readers when moving a writer. The payload's location changed in one
place and was read in another, and nothing tied the two together. A check that
compares an artifact against a path should fail loudly when that path is stale
rather than when the contents finally diverge, because a comparison that is
accidentally true is indistinguishable from one that is correct.
