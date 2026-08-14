#!/bin/sh
# Asks the bundle the only question that matters: can it answer? Three times in
# one afternoon the bundle came up, reported healthy, and could not — a runtime
# document missing the capability contract, a connector that was never
# registered, an account nobody had linked. Each looked identical from outside
# until a real message went through, so that is what this sends.
#
#   SMOKE_CHANNEL_ID=… SMOKE_SENDER_KEY_PATH=… SMOKE_AGENT_PUBKEY=… host/smoke.sh
#
# Needs the relay reachable and one member whose messenger key the bundle can
# resolve. Prints what came back and fails when nothing does.
set -e

: "${SMOKE_CHANNEL_ID:?set SMOKE_CHANNEL_ID to a conversation the agent is in}"
: "${SMOKE_SENDER_KEY_PATH:?set SMOKE_SENDER_KEY_PATH to a member's buzz secret}"
: "${SMOKE_AGENT_PUBKEY:?set SMOKE_AGENT_PUBKEY to the agent's buzz public key}"

SMOKE_RELAY_URL="${SMOKE_RELAY_URL:-ws://localhost:3000}"
SMOKE_WAIT_SECONDS="${SMOKE_WAIT_SECONDS:-90}"
SMOKE_QUESTION="${SMOKE_QUESTION:-Answer with one short line so a smoke test can see you are alive.}"
export SMOKE_RELAY_URL SMOKE_WAIT_SECONDS SMOKE_QUESTION
export SMOKE_CHANNEL_ID SMOKE_SENDER_KEY_PATH SMOKE_AGENT_PUBKEY

here="$(dirname "$0")"

echo "[smoke] asking the agent through ${SMOKE_RELAY_URL}"
bun run "${here}/smoke-ask.ts"

echo "[smoke] waiting up to ${SMOKE_WAIT_SECONDS}s for a reply from ${SMOKE_AGENT_PUBKEY}"
bun run "${here}/smoke-listen.ts"
