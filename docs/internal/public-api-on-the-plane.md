# The public API on the central plane

`api.intern.kim/v1` has to answer for everything a person can do through
internkim, under that person's own authority, including asking internkim
itself. This is how, and how without writing the API a second time.

## Three things that are already true

**The API is `handlePublicAPI`, and it is already a thin skin.**
`invokePublicTool` (`internal/admind/public_tool_gateway.go:215`) does four
things: look up the descriptor, check the permission, fill `Actor` and `Context`
from the token and the descriptor, then call `invokeCapabilityTool`, which POSTs
to capabilityd over a unix socket at `/v1/capabilities`. Blueclaw reaches the
same capabilityd. The agent and an HTTP caller already share the whole internal
path; there is nothing to unify, only something to keep.

**The ten scopes are three.** `publicAPIScopeRank` gives every scope a rank of
1, 2 or 3: `read`, everything else, then `destructive` and `admin`.
`publicToolScopeForDescriptor` maps fourteen side-effect classes onto those same
three. Read, write and delete is the model that is already running, with the
nine surplus names removed.

**The relay already asserts who is asking.** `serveWorkspace`
(`host/relay/relay.ts:313`) forwards to `http://127.0.0.1:18080` carrying
`X-INTERNKIM-REQUESTER-EMAIL`, and admind serves the call as that person. The
plane naming the caller to admind is an established move, used today by the
relay, capabilityd's flow tool and the web app's mail path.

## The shape

The plane does not implement the API. It carries a request to the one
implementation.

```
caller ── POST https://api.intern.kim/v1/tools/message_send/invoke
       │  Authorization: Bearer ik_…
       ▼
api worker      key → member, permission                       new
       │        discovery answered here; the rest goes on
       ▼
gateway DO      COMPANY_CALLS.callCompany, reusing CallLedger   new
       ▼
relay           person.api.request                              new
       │        unix socket, writes the requester header
       ▼
admind          handlePublicAPI, unchanged but for its listener
       ▼
capabilityd     where the agent goes too
```

What each piece is allowed to decide:

| | decides |
|---|---|
| api worker | who is calling, what their key permits, and the tool catalog |
| gateway | nothing; it carries bytes and matches an answer to its call |
| relay | nothing; it hands the request over under the resolved identity |
| admind | descriptor, permission, actor context, invocation |

Two of the four decide nothing at all. That is what keeps the second
implementation from growing: there is no place for it to live.

## The contract

Four pieces change one contract, so this is what each end may assume.

**Caller → api worker.** `Authorization: Bearer ik_…` and nothing else about
identity. Path and body are the public API's own, unchanged from the device.
The caller never names a person: the key already does.

**api worker → gateway.** A Cloudflare service binding, so a one-shot call has
no HTTP route at all: the gateway's public `fetch` handler answers `404` for
`/company/{id}/call`, and `CompanyCalls` is reachable only from a bound worker.
The api worker declares

```jsonc
"services": [
  { "binding": "COMPANY_CALLS", "service": "internkim-connection-gateway", "entrypoint": "CompanyCalls" }
]
```

and calls

```ts
const answer = await environment.COMPANY_CALLS.callCompany(companyID, {
  requestID: '…',
  capability: 'person.api.request',
  body: {
    method: 'POST',
    path: '/tools/message_send/invoke',
    query: '',
    permission: 'write',
    requester: 'someone@example.com',
    payload: {}
  }
});
```

`answer` is `{ requestID, status, body }`, and `status` is what the api worker
answers the caller with: `503` when no company server is connected, `504` when
the call outlives the gateway's 30 second bound, `429` when the company already
has 32 calls waiting on its one server socket, `409` when a call with the same
`requestID` is still in flight. A call naming no `requestID` or no `capability`
rejects rather than answering. The call names no member: the requester is in
`body`, and the routed call the company server receives carries no `memberID`
field.

**relay → admind.** `${admindSocketPath}` over unix, `/api/v1${path}${query}`,
carrying `X-INTERNKIM-REQUESTER-EMAIL` and `X-INTERNKIM-REQUESTER-PERMISSION`.
The relay is the only writer of both. It never forwards a header it received.

Every call that names a person goes that way, not only the public API. The
member capabilities (`person.memory.*`, `person.files.*`, `person.runs.*`,
`person.buzz.*`) are asked for on the same socket, carrying the email alone: a
member arrives through the gateway with no key, so there is no permission to
assert, and the paths they reach read none. What is left on `127.0.0.1:18080`
is the one call that names nobody, `/admin/api/directory/changed`.

**admind.** Listens on the unix socket as well as `127.0.0.1:18080`, and honours
the two headers only on the socket. On TCP they are ignored, exactly as if
absent, so a caller that finds the port gets whatever an anonymous caller gets.

## Who a call runs as

The key says. A personal key from `credential` resolves to exactly one member,
so the caller never names a person and has no way to name a different one. The
public API takes no requester header at all; the only header about identity is
`Authorization`.

There is no key that may act as somebody else. internkim working on a person's
behalf needs none: it runs on the company's own machine, reaches admind and
capabilityd directly, and the ledger already records the person as the actor.
Asking internkim to do something is a call made with the asker's own key, or a
message to it, and either way the authority is the asker's. A token can never do
more than its owner can, and this keeps that true by leaving no credential that
could.

The worker resolves the key to an address and passes that address on. Nothing
downstream repeats the resolution, and nothing downstream could: admind knows
people by address and has never heard of a key.

If server-to-server work for another person is ever genuinely needed, the
entitlement to reach for is the actor of a task run that is open right now, not
the company roster. The call is already going to admind, which owns the ledger,
so checking that a named task is running with a named actor costs nothing.

## Permissions

A key carries one of `read`, `write`, `delete`. They are a ladder: `delete`
implies `write` implies `read`, because a caller that cannot read cannot know
what to delete. New keys get `delete`, and every key issued before this gets
`delete`. Nothing is migrated and nothing is counted: no personal key reaches
the public API today, so there is no narrower key to widen.

This replaces the ten scope constants with the three ranks they already
collapsed into. `publicToolScopeForDescriptor` keeps doing its job and answers
with one of the three names. No per-tool permission table exists, because the
side-effect class on the descriptor already says which of the three a tool
needs.

## Files

`message_send` takes `attachments` as workspace paths, and no tool in the
catalog puts bytes into a workspace. So a caller can send back a file the
company already holds and cannot send one of their own.

Bytes must not travel in the call envelope. The relay answers at most
`largestMessageTheProPlanCarries`, three megabytes, and base64 leaves about
2.25 of raw file inside it. Reading an attachment already avoids the envelope:
`keptForReading` answers with an address in the company's asset bucket rather
than the file. Writing is that move mirrored.

`POST /v1/files` takes the body and the content type, puts it in the company's
asset bucket under the digest the bucket already addresses by, then makes one
ordinary call to the machine to materialise it into the workspace, and answers
the workspace path. That path is what `attachments` wants, so the caller learns
one concept and the message call is unchanged.

The bytes reach Supabase from the worker and never enter a capability call. The
call that follows carries a reference, so the three-megabyte ceiling never
applies to a file, and the company machine pulls from the bucket it already
holds a client for.

The materialising call is a capability like any other, which keeps the relay
deciding nothing: it downloads what it was named and posts it to admind's own
`/files/api/upload` over the same socket, under the same two requester headers,
then answers the directory it asked for joined with the name admind reports
back. The relay could not write the workspace itself: its unit runs as
`internkim` with `ProtectSystem=strict` and `ProtectHome=true`, a private home
belongs to that person's POSIX user, and on a device the workspace lives inside
the Blueclaw guest image. The directory is the requester's own personal root
from `/files/api/roots` with `inbox/api` under it, so the caller still names no
destination, and the filename it offered is reduced to its leaf. The worker
decides nothing about tools either; it uploads and asks.

A `message_send` naming those paths is the same move reversed. capabilityd runs
beside the guest with no identity to become, so it refuses a message whose files
nobody carried; admind reads each named path through Blueclaw as the person whose
key made the call and hands the content over in `transport.workspaceFiles`, which
is what the agent's own path does too.

## Asking the agent

`POST /v1/agent/messages` rides the same path to the same `handleAgentMessage`.
A caller who wants something no single tool covers describes it and lets the
agent choose, then reads `/v1/agent/replies`. It takes longer and needs no
machinery of its own.

## What this does not build

No REST implementation in TypeScript. No second token system: the personal key
is the credential, issued in the web app where the caller is already signed in.
`POST /v1/tokens` stays on the device path, because a machine cannot mint its
own first key and the plane's answer to that is a signed-in page.

## What is dangerous

**Loopback trust is what the unix socket is for.** `X-INTERNKIM-REQUESTER-EMAIL`
on `127.0.0.1:18080` today reaches workspace reads, and this design would let it
reach `message_delete`, `site_unserve` and `task_delete` as any employee.
Anything able to open that port would then be every person in the company: a
second service on the box, a request-forgery bug in one of them, or a task user
running a command that reaches loopback. Blueclaw runs code people asked for, on
that machine.

So the port stops being the gate. The relay reaches admind on a unix socket
owned by `internkim` with mode `0660`, the way admind already speaks to
capabilityd through `CapabilitySocketPath`, and admind honours the requester
headers only there. The relay runs as `internkim`, Blueclaw's service as
`blueclaw`, and the code it runs for people as `bc_person_*`, so the users are
already separate and the file mode is the whole boundary. What makes this
weaker than it looks is a second process running as `internkim`; that, and the
socket's mode, are what a test should watch.

**The plane becomes a required hop.** A device serves its own API today. After
this, `api.intern.kim` being down is every company's API being down, and one
caller waiting on a slow machine occupies a call slot on that company's single
server socket. The gateway answers `503` when no server is connected, waits
30 seconds for an answer before `504`, and refuses past 32 waiting calls. A
company that dislikes depending on the plane self-hosts, and a self-hosted
install needs none of this: its own admind already serves the API, so its
callers point straight at it.

## Order

The gateway, the relay and admind change one contract between them, so they ship
together. The gateway's one-shot call can land first on its own, because until
the relay knows `person.api.request` nothing calls it.
