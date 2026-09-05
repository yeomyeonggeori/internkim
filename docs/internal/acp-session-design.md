# ACP is the session boundary

Stage 3b of #1255. 3a put the company's catalog on `/api/v1/mcp` and taught
blueclaw to discover it; this stage replaces the transport a turn arrives on.

## The two ends

The relay (`host/relay`) is the ACP client. blueclaw is the ACP agent server.

blueclaw listens on a **new unix socket**, `/run/internkim/blueclaw-acp.sock`,
handed to both processes by `host/entrypoint.sh` as
`BLUECLAW_ACP_SOCKET_PATH`. Its admin API is HTTP over TCP and ACP is a
long-lived bidirectional JSON-RPC stream, so they cannot share a listener.

`github.com/coder/acp-go-sdk` is pinned at **v0.13.5**, already in blueclaw's
`go.mod`; the relay takes `@agentclientprotocol/sdk@1.4.0`. Both speak protocol
version 1. blueclaw implements `acp.Agent`; the restart path below adds the
optional `acp.AgentLoader`.

`internal/acpharness`, where blueclaw is the client driving an external agent,
is the harness axis (`harness-split-design.md` §3) and is untouched. This is
the opposite direction, in a new `internal/acpsession`.

## What `session/new` carries

`Cwd` is the requester's workspace root. `McpServers` names the relay's own
loopback `/mcp/<ticket>`. The relay forwards each call to the company's
`/api/v1/mcp` as the requester, minting the bearer through
`/api/agent/session` and again five minutes before it expires. A session lives
for weeks and a bearer for an hour, which is why the address is the relay's.

The rest rides on `_meta` under `kim.intern/session`:

| key | value |
|---|---|
| `requester` | person ID, email, display name, circles |
| `addressing` | platform, conversation ID and type, reply target, thread flag |
| `languageModel` | the rendered ladder's `tiers` and `embedding` |

The relay reads the ladder from the runtime document
`tools/render-company-runtime` writes, so `internal/modelladder` stays the only
place a model is named.

## A message becomes a prompt

chatd posts the inbound event to the relay's loopback listener, at `/inbound`
beside the arrivals it already takes. The body is the one chatd already builds
for `/connectors/{platform}/events`, unchanged, so neither path has a shape of
its own: the requester is read off `context.sender`, which chatd fills from the
adapter that knows the platform's users.

`/inbound` is durable, because the connectors inbox it replaces was. An event
is keyed `platform:conversationID:messageID`, written under the relay's state
directory, and answered **202** only once those bytes are on disk; a key
already there is answered 202 and kept once. chatd retries with backoff until
it gets that 202, and the relay drains the queue itself, bounded by an attempt
ceiling that logs the drop by key.

The relay opens one session per conversation and sends `session/prompt`. What
the conversation is rides `session/new`; what one message brought with it — its
id, its reply target, and chatd's visible context, so history, attachments and
whether the agent was named — rides the prompt's `_meta` under
`kim.intern/message`, because those change every time somebody writes. A
message in a room runs the addressing gate through `internal/inboundengagement`,
which both inbound paths now call.

blueclaw runs a real turn: `internal/acpsession` builds the
`AgentTurnRequest` the connector runtime builds today and calls
`TaskLauncher.Launch`. Checkpoints go out as `agent_thought_chunk`, and one
naming a tool also as a `tool_call` update; the finish message goes out as
`agent_message_chunk`, so a client that concatenates message chunks the way ACP
says to gets the answer and none of the commentary.
`PromptResponse.StopReason` closes the turn, and the relay posts the reply
through `forwardToChatd`.

## Permission

A call `approvalgate` holds today pauses the run, records the held call, and
asks through the reply path. Over ACP the gate calls `RequestPermission` on the
session and blocks on the answer.

The relay posts the question to the requester on the messenger and reads the
next message in that conversation as the answer. What those words mean is
blueclaw's to decide: the relay hands them back over the extension method
`_kim.intern/approvalReply`, which runs the same turn router that reads a
confirmation reply on the connectors path. An answer the router cannot read
declines, so no call runs that nobody agreed to.
`allow_once` and `reject_once` are always offered; a tool carrying an approval
scope also gets `allow_always`.

`approvalgate.Gate` gains one seam, a `PermissionAsker`. A turn that arrived
over ACP has one and blocks; a turn that arrived through the connectors keeps
returning `ApprovalDecisionHeld`. The gate writes the same pending-call,
confirmation, ask, held-call and decided events on both paths.

An approved call then has to say so to capabilityd, which refuses any
approval-gated descriptor that cannot show the requester agreed. Until 3b that
claim was spelled `isApprovalContinuation`, a property of the *turn*, because a
held call could only come back on a later one. It is now also spelled per call:
the invocation carries `approvedCallID`, the held-call token
`TaskEventApprovalExecuted` records. capabilityd takes either, and refuses a
call carrying neither.

## Restart

The held call is persisted exactly as today, as `agentcontract.HeldCall` on the
task event ledger. Its id is derived from it — `approvalgate.HeldCallID`, a
digest of `CanonicalToolCallKey` — so it is the same value in the ledger, in
`RequestPermissionRequest.ToolCall.ToolCallId` and in `approvedCallID` on the
wire, and a restarted daemon recomputes it from the record it already has.

So that a restart finds something, the ACP gate records the held call and
pauses the run to `waiting_approval` before it blocks on `RequestPermission`,
and advances the run again on the answer. The events are the ones the
connectors path already writes; only their order changed. A daemon killed
mid-question leaves what a connectors turn would have left, and the
interrupted-run auto-resume passes it over: a run that is waiting was never
interrupted.

When blueclaw restarts, the relay notices the socket end, opens a new one, and
calls `session/load` with the session id it holds and the same `_meta`, so
blueclaw persists nothing about the session. blueclaw reads the ledger, finds
every task run in `waiting_approval` for that conversation, and re-issues
`RequestPermission` with the same held-call id — after answering the load,
since the client is blocked on that response. The relay deduplicates on that
id, holding the person's raw words and not the agent's reading of them, so an
answer it already has goes to the new agent to read for itself. An approving
answer relaunches the run as an approval continuation.

The relay's own restart is symmetric: `held-question-store.ts` persists each
question beside the inbound queue and reloads it at boot, so a re-issued call
is answered from the store, or waits, without asking again.

## What 3b leaves for 3c

`internal/connectors` stays as the second inbound path, behind a new blueclaw
flag `-inbound` taking `connectors` or `acp`. The binary's default stays
**`connectors`** and `host/entrypoint.sh` passes `-inbound acp`, so which path
a company runs is decided where a company is brought up, and the frozen device
passes no such flag. Under `acp`, `/connectors/{platform}/events` answers 409
naming the flag, so a chatd still posting is loud rather than quietly doubling
the turn.

3c stops the plane stamping: `--print-capabilities` prints capabilityd's
address, and a document that stamps nothing takes its descriptors from
`/v1/capabilities` and pins no identity. The package, the flag,
`internal/protocolidentity`, the per-turn audit, admind's restamp and the
pairing rule are the frozen device's, and stay until it is retired.

## What is not built yet

The ladder on `_meta`. Carrying a value nothing reads is dead configuration, so
`session/new` sends the requester and the addressing only.
