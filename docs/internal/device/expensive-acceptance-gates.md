# Expensive acceptance gates (real Mattermost)

The device path is frozen; see [README.md](./README.md). Mattermost is not part
of that freeze, it is being removed, so this gate covers what still runs rather
than what to build on.

- Run one scenario per invocation and keep the VM for autopsy:
  `./internkim test expensive --maximum-model-tier low --scenario <name>
  --auto-confirm --keep --retry-once`. Rerun on the same fleet without
  reprovisioning: add `--skip-provisioning --run-id <existing runID>`.
- Never run two expensive scenarios concurrently against one fleet: each
  run tears down and recreates the shared SSH tunnel recorded in the run
  directory, killing the other run's connection mid-flight. Chain
  scenarios sequentially with `tools/run-expensive-chain --run-id <runID>
  [scenario ...]`; do not hand-roll the chain with ad-hoc shell. The tool
  is single-instance (PID file, no string matching against process lists)
  and writes each run to its own
  `.local/local-fleet/runs/<runID>/chain-<stamp>.log` with a
  `chain-current.log` symlink, so a dead run's lingering append descriptor
  can never contaminate a new run's log and a log watcher never replays a
  previous run's verdicts.
- Reusing the kept fleet is the default; each fresh provision costs ~10
  minutes and several GB of host disk. Scenario or harness-only changes
  rerun with `--skip-provisioning --run-id`; Go changes push in place with
  `./internkim dev fleet reprovision --config
  .local/local-fleet/runs/<runID>/config.json` after committing. Create a
  fresh VM only when the current one is suspect (guest postgres fsync
  death, broken provisioning). Retire a fleet by powering it off (`lab
  vm-ssh ... 'sudo poweroff'`); the next run's reaper removes stopped
  fleets and reclaims their disk. Prune old run evidence with
  `tools/prune-expensive-artifacts <keepCount>` (archives to a verified
  sibling tarball before deleting; never hand-roll this with ad-hoc
  shell).
- Preflight before every run: `df -h /` must show 15Gi+ free (Mattermost
  install fails opaquely below that), and `container ls -a` must show no
  leftover `internkim-e2e-expensive-*` container (stop+rm leftovers first;
  never touch VMs from other sessions).
- Launch the runner so its lifetime is tracked by your harness. Never pipe the
  runner through `tail`/`head` (they buffer everything and the output file
  stays empty for the whole run) and never orphan it with bare `nohup` from a
  tool call.
- The ONLY pass/fail signal is a line-anchored
  `^(✓|✗) expensive scenario <name>`. Inner playwright test names also contain
  the words "expensive scenario", so an unanchored grep produces false DONE
  verdicts.
- Artifacts: `.artifacts/expensive/<runID>/<scenario>/diagnostics/result.json`
  (per-step status), `diagnostics/events/step-XX.json` (full task event
  ledger), `evidence/step-XX/*.png` (acceptance screenshots),
  `attempt-2/` (retry artifacts). Guest admin API for live autopsy:
  `./internkim lab vm-ssh --config .local/local-fleet/runs/<runID>/config.json
  -- "curl -s 127.0.0.1:8080/admin/api/task"` and
  `/admin/api/task/detail?taskRunID=<id>`.
- Failure triage order, always: (1) read the step's event ledger, (2) classify
  the failing layer — product (agent/runtime), harness (stale scenario
  expectation), or infra (lines prefixed `infra failure`/`infra-suspect`,
  provisioning, VM postgres/disk) — (3) fix that layer at the root, (4) rerun
  only to verify the fix. Never rerun an unexplained failure, and if the
  failure reason is missing from the log, fix that reporting gap first.
- Scenario JSON expectation rules: `expectedTaskStatus`, approval lifecycle
  events (`approval.pending_call`, `approval.executed`,
  `confirmation.requested`), and user-visible side-effect results stay
  required; implementation-detail expectations (exact tool output content,
  internal event bodies) get `"advisory": true`. Never pin an exact model
  (the tier ceiling plus the capability transport is the contract; the runtime
  may ladder within the ceiling). Never require a `*.list` call before a
  hint-based mutation (`taskHint` tools resolve server-side without listing).
- A gate binary/payload pairs with the Blueclaw HEAD it was built from: after
  any `.dependency/blueclaw` commit, rebuild `make prepare-blueclaw-payload`
  before launching, or the run ships the old agent.

