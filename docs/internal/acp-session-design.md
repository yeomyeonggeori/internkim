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
optional `acp.AgentLoader`. The SDK lacks nothing this design needs.

`internal/acpharness`, where blueclaw is the client driving an external agent,
is the harness axis (`harness-split-design.md` §3) and is untouched. This is
the opposite direction, in a new `internal/acpsession`.

## What `session/new` carries

`Cwd` is the requester's workspace root. `McpServers` carries one HTTP entry:
the company's `/api/v1/mcp` and a per-session bearer minted for the requester,
which is what retires capabilityd's `/v1/mcp` carrier, shipped by 3a as
scaffolding.

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
beside the arrivals it already takes. The relay opens one session per
conversation and sends `session/prompt` carrying the message text.

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
task event ledger, and nothing new is written down. Its id is derived from it —
`approvalgate.HeldCallID`, a digest of `CanonicalToolCallKey` — so it is the
same value in the ledger, in `RequestPermissionRequest.ToolCall.ToolCallId` and
in `approvedCallID` on the wire, and a restarted daemon recomputes it from the
record it already has.

When blueclaw restarts, the relay reconnects and calls `session/load` with
the session id it holds. blueclaw reads the ledger, finds every task run in
`waiting_approval` for that conversation, and re-issues `RequestPermission`
with the same held-call id. The relay deduplicates on that id: a question it
already asked is not asked again, and an answer it already has is returned at
once.

## What 3b leaves for 3c

`internal/connectors` stays as the second inbound path, behind a new blueclaw
flag `-inbound` taking `connectors` or `acp`, defaulting to **`connectors`**.
Under `acp`, `/connectors/{platform}/events` answers 409 naming the flag, so a
chatd still posting is loud rather than quietly doubling the turn. 3c deletes
the package, the flag, `internal/protocolidentity`, the runtime.json
`capabilities` block and its stamp sources, and the pairing rule.

## The first slice

Only this: blueclaw serves ACP on the socket; `session/new` and
`session/prompt` produce a real turn whose reply streams back;
`RequestPermission` round-trips for `message_send`; and a plane scenario drives
one Buzz direct message through the relay end to end, reading the delivery from
the recipient's inbox. Only the plane posts to `/inbound` so far; pointing a
company's chatd at it comes with `-inbound acp`.

Three pieces are written here and built in later slices: the MCP endpoint and
per-session token on `session/new`, the ladder on `_meta`, and restart survival
through `session/load`. Carrying a value nothing reads is dead configuration,
so `session/new` carries the requester and the addressing and nothing else.
