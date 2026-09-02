# The local fleet

The Linux gate for both paths. A run starts a local central plane and joins the
VM to it before anything else, so the plane's own wiring is under test here too;
the `buzz-*` scenarios are the messenger gate `AGENTS.md` keeps, and they run
nowhere else. What is frozen is the device half of what the VM exercises: OTA,
the guest's ext4 workspace. See
[README.md](./README.md) for what the freeze means.

The VM is a disposable Apple Container running the same Blueclaw guest a device
runs, so a change that only breaks on Linux breaks here first.

- After local simulation passes, verify executable and Linux permission behavior
  with `./internkim dev fleet run --without-mattermost --scenario <name>`.
- Treat Local Fleet VM verification as the required pre-deploy Linux/runtime gate for
  agent execution that touches `shell`, `bun`, `uv`, Python dependency wrappers,
  POSIX users/groups, or workspace permissions.
- The guest's `/workspace` is `/var/lib/blueclaw/workspace.ext4`, attached to
  the VM. `/root/.blueclaw/workspace` on the host is a different tree, not a
  mount of that image. A host daemon that writes there and answers with a
  `/workspace` path has told the agent about a file that is not there; hand the
  bytes to blueclaw and let it write them as the person instead.
- The disposable local fleet is an Apple Container VM (`internkim-e2e-<runID>`) with
  its config at `.local/local-fleet/runs/<runID>/config.json`; Blueclaw runs as a
  cloud-hypervisor guest inside it, so skill/Blueclaw/runtime changes only reach it
  through a reprovision. Push working-tree changes onto the running VM with
  `./internkim dev fleet reprovision`; it reprovisions in place with the required
  `GO_MOD_CACHE` override and resets the policy people and guest Postgres.
- Reprovision (and OTA) deploy Blueclaw by release SHA (`.dependency/blueclaw` HEAD):
  the guest runs `.blueclaw/runtime/releases/<sha>/bin/blueclaw` and the deploy is
  idempotent on that SHA. An UNCOMMITTED Blueclaw Go change builds a new binary but
  stamps the same HEAD SHA, so the deploy sees "already deployed" and SKIPS it — the
  guest keeps the old binary even though the working tree and host payload have the fix.
  COMMIT Blueclaw Go changes (new SHA) before reprovision, then verify by grepping the
  guest binary: `lab vm-ssh --config <cfg> 'sudo grep -c "<changed string>"
  /root/.blueclaw/workspace/.blueclaw/runtime/current/bin/blueclaw'`. Skills are python
  on the virtiofs share (`/root/.blueclaw/workspace/skills`) and hot-push live (write +
  restart Blueclaw, no reprovision); only the Go binary needs the commit+reprovision.
  `./internkim test` does NOT build from the working tree (it uses the committed/artifact
  payload), so it never carries uncommitted changes.
- Never run `./internkim lab` with no subcommand: it creates and starts a separate
  scratch `internkim-lab` container. Reach the fleet only through
  `./internkim lab vm-ssh --config <cfg>` and `./internkim lab vm-ip --config <cfg>`.
  Do not `container stop`/`container delete` the active fleet container. Confirm the
  fleet with `container ls` (`internkim-e2e-<runID>` present); if it is gone it was
  destroyed, and a fresh one comes up via `./internkim test cheap --keep`.
- Do not redeploy agent, Blueclaw, runtime, skill, or terminal-execution changes
  until the relevant Local Fleet run produces the intended result. If Local Fleet verification
  fails, fix the behavior or explicitly report the unresolved failure instead of
  proceeding to deployment.
- Do not replace this gate with another executor unless the task explicitly asks
  for a different executor model. Nothing here runs on Docker: the fleet VM is an
  Apple Container and the guest is cloud-hypervisor.
- For web, admind, capabilityd, Mattermost connector, or cross-service behavior
  changes, verify the current checkout through the local fleet with
  `./internkim dev fleet run` or a narrower
  `./internkim dev fleet run --scenario <name>`.
- When claiming a user-visible fix, prefer
  `./internkim dev fleet verify-regression --base main --scenario <name>` so the
  same scenario fails on the base revision and passes on the current checkout.
- Run real Mattermost smoke only after the virtual-session and Mattermost-free
  Linux gates pass, and clean up after it the way `AGENTS.md`'s Runtime Test
  Hygiene says. Disposable local fleet runs stop and remove their VM by default while keeping
  gitignored evidence under `.local/local-fleet/runs/<runID>` and
  `.artifacts/local-fleet/<runID>`; use `./internkim dev fleet reset` after
  `--reuse` runs.
- `run` creates a run-scoped VM, state directory, and localhost tunnel ports,
  then removes them after the scenario. `--keep` leaves the VM, logs, Mattermost
  posts and users, and state for debugging, and prints the cleanup command.
  `--reuse` takes the shared local fleet instead; pair a reused run with
  `dev fleet reset` or `dev fleet down`. `--without-mattermost` still runs inside
  the Linux VM and uses the checkout's Linux toolchain without starting Kim
  services or Mattermost ingress.
- The scenarios are registered in `internal/localfleet/service.go`. Read that
  map; a list kept anywhere else goes stale. Keep runtime-invariant
  scenarios deterministic with a scripted model and use `--live-llm` for model
  judgment. Create Mattermost users, channels, posts, and Blueclaw state through
  a lease, and clean the lease unless you are debugging. Production, pilot,
  Jetson, Cloudflare, and direct deploy secrets never go into `.local/local-fleet`.
