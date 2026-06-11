# Local Fleet Scenarios

Local Fleet scenarios verify the current checkout against a Tart-hosted local
InternKim fleet. They are for localhost-only validation, not deployment.

## Commands

```bash
./internkim dev fleet up
./internkim dev fleet run --recipe predeploy-gate
./internkim dev fleet run --scenario mattermost-bot-invited
./internkim dev fleet run --scenario mattermost-direct-message-send
./internkim dev fleet verify-regression --base main --scenario regression-proof
./internkim dev fleet reset
```

`./internkim sim gate` is a compatibility alias for the `predeploy-gate` recipe.
New automation should call `./internkim dev fleet ...` directly.

## Scenario Rules

- Keep scenarios deterministic by default.
- Use fake or cassette LLM behavior unless the scenario explicitly requires
  `--live-llm`.
- Create Mattermost users, channels, posts, and Blueclaw state through a lease.
- Clean the lease by default; keep artifacts only when debugging.
- Do not copy production, pilot, Jetson, Cloudflare, or direct deploy secrets
  into `.local/local-fleet`.

## Current Scenarios

- `mattermost-bot-invited`: verifies real Mattermost ingress and bot replies.
- `mattermost-direct-message-send`: verifies real Mattermost DM recipient
  resolution, approval, platform.message.send, and recipient DM delivery.
- `web-backed-ui`: verifies UI behavior against local fleet admind.
- `regression-proof`: verifies base-fails/current-passes regression plumbing.
