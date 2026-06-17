# InternKim multi-tenant container PoC (M1, Firecracker-free)

Runs N independent InternKim tenants as lightweight containers on Apple Silicon
that cannot nest virtualization (M1/M2). Each tenant runs Blueclaw in
**direct/native mode** (no Firecracker, no supervisor, no systemd), sharing one
Postgres and one Mattermost.

## Architecture

- **Shared Postgres** — one database per tenant (`tenant_01`..`tenant_NN`) plus
  `mattermost`.
- **Shared Mattermost** — one team and one agent identity per tenant. The agent's
  bot token is the tenant boundary: the connector only sees channels that token
  can read.
- **Per-tenant container** — `internkim-capabilityd` + `blueclaw` started directly
  by `tenant/entrypoint.sh`, connected over a unix socket. `terminal.mode=native`,
  graphiti disabled, POSIX synchronization skipped, LLM via OpenRouter.

Measured idle footprint: ~8 MiB per tenant; whole 10-tenant stack ~1.4 GiB
including Postgres and Mattermost.

## Layout

- `configgen/` — emits direct-mode `runtime.json` + `policy.json` (uses
  `BlueclawRuntimeConfigDocumentWithOptions(DirectExecution: true)`).
- `tenant/` — tenant image (`Dockerfile`, `entrypoint.sh`, `bin/`, `migrations/`).
- `infra/docker-compose.yml` — Postgres + Mattermost.
- `generate-configs.sh` / `render-compose.sh` / `provision-mattermost.sh`.
- `config/`, `secrets/`, `tenants.generated.yml` — generated, gitignored.

## Run (Mac Studio, colima)

```bash
colima start --cpu 6 --memory 20 --disk 60

# build linux/arm64 binaries on a dev machine, copy into tenant/bin/:
#   blueclaw, blueclaw-posix-helper, internkim-capabilityd
# copy .dependency/blueclaw/migrations -> tenant/migrations
# place the OpenRouter key at secrets/openrouter-key

export TENANT_COUNT=10
bash generate-configs.sh                       # run where the Go module lives
docker compose -f infra/docker-compose.yml up -d
bash provision-mattermost.sh                   # teams, agent users, bot tokens
docker build -t internkim-poc-tenant:latest tenant
bash render-compose.sh
docker compose -f tenants.generated.yml up -d
```

## Not yet wired

- External addresses (Cloudflare tunnel per team).
- Terminal toolchain (bun/uv/python) in the tenant image — only needed for
  `terminal.run` tasks; the chat loop does not require it.
- POSIX per-person isolation (bake the `blueclaw` base user into the image and
  restore `posixHelperPath` to re-enable).
- arm64 Mattermost image (current `10.5` tag is amd64, runs under emulation).
