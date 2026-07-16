# Local Fleet Scenarios

Local Fleet scenarios verify the current checkout against an apple/container-hosted local
InternKim fleet. They are for localhost-only validation, not deployment.

## Commands

```bash
./internkim dev fleet run
./internkim dev fleet run --scenario mattermost-direct-message-send
./internkim dev fleet run --without-mattermost --scenario dm_send_confirm_acceptance
./internkim dev fleet run --scenario mattermost-bot-invited
./internkim dev fleet verify-regression --base main --scenario regression-proof
./internkim dev fleet run --keep --scenario mattermost-manual
./internkim dev fleet run --keep --scenario mattermost-docx-attachment
./internkim dev fleet run --reuse --scenario mattermost-direct-message-send
```

`./internkim sim gate` is a compatibility alias for `./internkim dev fleet run`.
New automation should call `./internkim dev fleet run` directly.

For a browser-ready manual Mattermost session, run:

```bash
./internkim dev fleet run --keep --scenario mattermost-manual
```

This provisions the full Blueclaw runtime, creates an invited test account,
opens its direct message with InternKim, verifies real typing and reply events,
removes the verification posts, and prints
the username, password, and direct-message channel to use for continued manual
testing. It also prints the localhost Mattermost URL and cleanup command. No
separate invite or test-message setup is required. The local-only login is
always `test` / `test`.

`run` creates a run-scoped VM, state directory, and localhost tunnel ports by
default, then removes them after the recipe or scenario. Pass `--keep` to leave
the VM, logs, Mattermost posts/users, and state for debugging. The command
output prints the cleanup command. Pass `--reuse` only when you need the shared
local fleet for manual debugging; pair reusable runs with `dev fleet reset` or
`dev fleet down` when you are done.

Use `--without-mattermost` with a virtual-session scenario when Mattermost itself
is not under test. This still runs inside the Linux VM and uses the checkout's
Linux toolchain, but it does not start Kim services or rely on Mattermost ingress.

Use `--keep` for user-visible Mattermost regressions that need screenshot proof.
After the scenario passes, open the preserved Mattermost URL, log in to the
printed requester account, and capture the DM or thread showing
the original prompt, Kim's final reply, and the native attachment card or public
URL. Save screenshots and downloaded files under `.local/local-fleet/runs/<run-id>/`.
These files are local evidence only; do not commit them. Clean the kept VM and
Mattermost artifacts after review.

## Scenario Rules

- Keep runtime-invariant scenarios deterministic with a scripted model.
- Use `--live-llm` for model judgment and AI SDK acceptance.
- Create Mattermost users, channels, posts, and Blueclaw state through a lease.
- Clean the lease by default; keep artifacts only when debugging.
- Do not copy production, pilot, Jetson, Cloudflare, or direct deploy secrets
  into `.local/local-fleet`.

## Current Scenarios

- `mattermost-bot-invited`: verifies real Mattermost ingress and bot replies.
- `mattermost-direct-message-send`: verifies real Mattermost DM recipient
  resolution, approval, platform.message.send, and recipient DM delivery.
- `mattermost-manual`: prepares and preserves an invited browser login with a
  verified typing event and direct message reply from InternKim.
- `mattermost-docx-attachment`: verifies a real Mattermost prompt, DOCX
  generation, native `file.attach` delivery, and local attachment download.
- `--without-mattermost --scenario <virtual-session>`: verifies Blueclaw's Linux
  execution path without a Mattermost server.
- `web-backed-ui`: verifies UI behavior against local fleet admind.
- `regression-proof`: verifies base-fails/current-passes regression plumbing.
