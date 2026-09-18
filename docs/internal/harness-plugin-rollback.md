# Rolling back the shared plugin release

What shipped for internkim#1789 spans three deployables that carry one
contract, and each is rolled back by a forward change on `main`.

| Deployable | Carries | Rolled back by |
| --- | --- | --- |
| Device release (`internkim`, `admind`, `capabilityd`, `blueclawPayload`, `blueclawSupervisor`, `chatd`, `skills`, `web`) | the catalog, its capabilityd handlers, Blueclaw without native schedule tools, the plugin checkout as `skills` | a revert commit on `main`, then `tools/deploy-main` |
| Cloudflare Pages `internkim` | the public MCP server and `/api/v1/tools` | `web/scripts/deploy-pages.ts --production` from the reverted `main` |
| Plugin repository `internkim-plugin` | `plugin.json`, `mcp.json`, `skills/` | the `.dependency/internkim-plugin` pointer in the same revert |

## Why a revert and not an older release

`internkim deploy` refuses a tree that does not contain `origin/main` and a
component set that would leave the device holding components the tree has
moved past; `tools/deploy-main` also refuses a device whose running revision
is absent from `origin/main` (`git merge-base --is-ancestor`).
Both guards exist so that a stale branch can never replace what every
company signs in at. A rollback therefore is: revert the pull request on
`main`, let the checks run, merge, and deploy the reverted `main` as one
set. `deploy-pages.ts` has the same shape: it refuses to replace a newer
production build unless `--replace-newer` is passed, and a reverted `main`
is newer, so the flag is not needed.

## The commits to revert

- internkim#1819 (`342d44d84`): the catalog-owned schedule tools, the
  capabilityd forwarding, the plugin gate, and the Blueclaw pointer
  `f41ae448`.
- internkim#1823: the Claude Code marketplace entry.
- internkim#1830 (`b3b1406e3`): the Local Fleet scenario's matcher. Only
  needed if the scenario is expected to pass on the reverted tree.
- blueclaw#381 in `.dependency/blueclaw`: restores Blueclaw's native
  `schedule_*` tools. Revert it there first, then point the submodule at
  the revert.

## What cannot be rolled back

Blueclaw migration 035 renamed the schedule table and has no down
migration. A Blueclaw revision from before 035 reads the old table name
and fails at startup against a database that has run 035. The revert of
blueclaw#381 keeps 035 (it only restores the tools on top of it), so the
ordinary rollback is safe; putting a pre-035 Blueclaw on a device means
restoring its Postgres from the backup taken before the release, and that
loses the schedules created since.

## Trigger conditions

Roll back when, on the device, a schedule request ends `failed` or
`blocked` with `tool.schedule_*` errors in the run's event ledger
(`/admin/api/run/detail?taskRunID=…`), or `/admin/api/schedule` answers
5xx, and the cause is not an upstream model outage (compare with
`/admin/api/health` and the capabilityd log first; internkim#1821 describes
what a decisions outage looks like). Roll back the Pages deployment when
`https://api.intern.kim/v1/mcp` answers `tools/list` with an error for a
signed-in member, or a client that worked before the release cannot call a
tool it lists.

## The order

1. Revert on `main` (device and Pages read the same commit).
2. Deploy the device: `make build`, `make prepare-blueclaw-payload`,
   `cd web && bun run build:board`, then `tools/deploy-main`. Read
   `/admin/api/updates/status` and `/admin/api/health` afterwards; the
   deploy's own green exit is not the evidence.
3. Deploy Pages from the same tree: `cd web && bun run build && bun run
   scripts/deploy-pages.ts --production`.
4. Tell the plugin's clients nothing: the plugin repository's `main` is
   what the `skills` component ships, and the public server serves whatever
   catalog the Pages deployment carries, so a client sees the rollback on
   its next `tools/list`.

## Test resources after the release

- Local Fleet `20260918t124540-a5f0c309` was kept for autopsy. Its VM
  (`internkim-e2e-20260918t124540-a5f0c309`) is retired with
  `sudo poweroff` inside the guest; the next run's reaper removes it.
  Evidence stays under `.artifacts/local-fleet/20260918t124540-a5f0c309/`
  (`schedule-through-the-catalog-4`, `firing-schedules-nothing-8`).
- The scenario deletes every schedule it created in its `finally`; the
  fleet's people and database are disposable with the VM.
- No task, schedule, message or person was created on the device or on any
  customer plane to verify the release.
